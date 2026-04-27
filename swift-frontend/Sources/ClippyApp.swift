import SwiftUI
import WebKit
import Cocoa
import Foundation

// ── App Delegate ──
class AppDelegate: NSObject, NSApplicationDelegate {
    var statusItem: NSStatusItem!
    var panel: NSPanel?
    var webView: WKWebView?
    var backendProcess: Process?
    var clickOutsideMonitor: Any?
    var clickLocalMonitor: Any?
    var hotkeyGlobalMonitor: Any?
    var hotkeyLocalMonitor: Any?
    var clipboardMonitor: Timer?
    var lastImageHash: Int = 0

    private let backendURL: URL = {
        let appBundleURL = Bundle.main.bundleURL
        return appBundleURL.appendingPathComponent("Contents/Resources/go-backend/clippy-server")
    }()

    private var uiDir: URL {
        if let resourcesPath = Bundle.main.resourcePath {
            return URL(fileURLWithPath: resourcesPath).appendingPathComponent("ui-prototype")
        }
        return URL(fileURLWithPath: "/Users/qq/workspace/clippy-v2/ui-prototype")
    }

    private var dbPath: String {
        let appSupport = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first!
        let dir = appSupport.appendingPathComponent("Clippy")
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("clippy.db").path
    }

    // ── Launch ──
    func applicationDidFinishLaunching(_ notification: Notification) {
        startBackend()
        setupStatusItem()
        setupPanel()
        setupGlobalHotkey()
        startClipboardMonitor()
    }

    func applicationWillTerminate(_ notification: Notification) {
        stopClipboardMonitor()
        stopBackend()
        cleanupMonitors()
    }

    // ── Clipboard Image Monitor ──
    func startClipboardMonitor() {
        clipboardMonitor = Timer.scheduledTimer(withTimeInterval: 0.5, repeats: true) { [weak self] _ in
            self?.checkClipboardForImage()
        }
        print("📋 Clipboard image monitor started (0.5s interval)")
    }

    func stopClipboardMonitor() {
        clipboardMonitor?.invalidate()
        clipboardMonitor = nil
    }

    private func checkClipboardForImage() {
        let pasteboard = NSPasteboard.general

        guard let types = pasteboard.types else { return }
        guard types.contains(.png) || types.contains(.tiff) else { return }

        guard let imageData = pasteboard.data(forType: .png) ?? pasteboard.data(forType: .tiff) else { return }

        // Dedup by hash
        let hash = imageData.hashValue
        if hash == lastImageHash { return }
        lastImageHash = hash

        // Convert TIFF to PNG if needed
        var pngData = imageData
        if pasteboard.data(forType: .tiff) != nil && pasteboard.data(forType: .png) == nil {
            if let bitmap = NSBitmapImageRep(data: imageData),
               let converted = bitmap.representation(using: .png, properties: [:]) {
                pngData = converted
            }
        }

        // Save to temp file
        let tempDir = FileManager.default.temporaryDirectory
        let tempFile = tempDir.appendingPathComponent("clippy_upload_\(UUID().uuidString).png")
        do {
            try pngData.write(to: tempFile)
        } catch {
            print("❌ Failed to save temp image: \(error)")
            return
        }

        // Upload to Go backend
        uploadImage(fileURL: tempFile)

        // Clean up temp file after a delay
        DispatchQueue.main.asyncAfter(deadline: .now() + 2) {
            try? FileManager.default.removeItem(at: tempFile)
        }

        print("📤 Image detected on clipboard, uploading...")
    }

    private func uploadImage(fileURL: URL) {
        let url = URL(string: "http://localhost:5100/api/clips/image")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"

        let boundary = "Boundary-\(UUID().uuidString)"
        request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")

        var body = Data()

        // Add file
        let filename = fileURL.lastPathComponent
        body.append("--\(boundary)\r\n".data(using: .utf8)!)
        body.append("Content-Disposition: form-data; name=\"image\"; filename=\"\(filename)\"\r\n".data(using: .utf8)!)
        body.append("Content-Type: image/png\r\n\r\n".data(using: .utf8)!)

        do {
            let fileData = try Data(contentsOf: fileURL)
            body.append(fileData)
        } catch {
            print("❌ Failed to read temp file: \(error)")
            return
        }

        body.append("\r\n--\(boundary)--\r\n".data(using: .utf8)!)
        request.httpBody = body

        URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                print("❌ Upload failed: \(error)")
                return
            }
            if let data = data, let resp = String(data: data, encoding: .utf8) {
                print("✅ Image uploaded: \(resp)")
            }
        }.resume()
    }

    // ── Backend ──
    func startBackend() {
        let process = Process()
        process.executableURL = backendURL

        // Fixed data directory: ~/Library/Application Support/Clippy/
        let appSupportDir = NSHomeDirectory() + "/Library/Application Support/Clippy"
        let imagesDir = appSupportDir + "/images"
        try? FileManager.default.createDirectory(atPath: imagesDir, withIntermediateDirectories: true)

        process.arguments = [
            "-port", "5100",
            "-data", appSupportDir,
            "-images", imagesDir,
            "-static", uiDir.path
        ]

        let logDir = FileManager.default.temporaryDirectory
        let stdoutLog = logDir.appendingPathComponent("clippy-backend-stdout.log")
        let stderrLog = logDir.appendingPathComponent("clippy-backend-stderr.log")
        FileManager.default.createFile(atPath: stdoutLog.path, contents: nil)
        FileManager.default.createFile(atPath: stderrLog.path, contents: nil)
        process.standardOutput = try? FileHandle(forWritingTo: stdoutLog)
        process.standardError = try? FileHandle(forWritingTo: stderrLog)

        do {
            try process.run()
            backendProcess = process
        } catch {
            print("❌ Backend failed: \(error)")
        }
    }

    func stopBackend() {
        backendProcess?.terminate()
        backendProcess = nil
    }

    // ── Status Bar ──
    func setupStatusItem() {
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.squareLength)
        if let button = statusItem.button {
            button.image = NSImage(systemSymbolName: "doc.on.clipboard", accessibilityDescription: "Clippy")
            button.action = #selector(statusItemClicked)
            button.target = self
        }
    }

    @objc func statusItemClicked() {
        togglePanel()
    }

    // ── Panel ──
    func setupPanel() {
        let panelWidth: CGFloat = 400
        let panelHeight: CGFloat = 520

        panel = NSPanel(
            contentRect: NSRect(x: 0, y: 0, width: panelWidth, height: panelHeight),
            styleMask: [.titled, .fullSizeContentView],
            backing: .buffered,
            defer: true
        )

        panel?.titlebarAppearsTransparent = true
        panel?.titleVisibility = .hidden
        panel?.isMovableByWindowBackground = true
        panel?.level = .floating
        panel?.backgroundColor = .clear
        panel?.hasShadow = true
        panel?.isOpaque = false
        panel?.collectionBehavior = [.canJoinAllSpaces, .stationary]
        panel?.animationBehavior = .utilityWindow
        panel?.hidesOnDeactivate = false

        // Content
        let config = WKWebViewConfiguration()
        config.preferences.setValue(true, forKey: "developerExtrasEnabled")

        let webView = WKWebView(frame: panel!.contentView!.bounds, configuration: config)
        webView.autoresizingMask = [.width, .height]
        webView.allowsBackForwardNavigationGestures = false
        webView.setValue(false, forKey: "drawsBackground")

        panel?.contentView?.addSubview(webView)
        self.webView = webView

        let htmlFile = uiDir.appendingPathComponent("index.html")
        webView.loadFileURL(htmlFile, allowingReadAccessTo: uiDir)
    }

    // ── Global Hotkey (NSEvent) ──
    func setupGlobalHotkey() {
        let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue(): true] as CFDictionary
        guard AXIsProcessTrustedWithOptions(options) else {
            print("⚠️ Accessibility not enabled - hotkey will not work")
            return
        }

        // Global monitor: catches Cmd+Shift+V in OTHER apps
        hotkeyGlobalMonitor = NSEvent.addGlobalMonitorForEvents(matching: .keyDown) { [weak self] event in
            self?.handleHotkey(event)
        }

        // Local monitor: catches Cmd+Shift+V in OUR app
        hotkeyLocalMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            self?.handleHotkey(event)
            return event
        }

        print("✅ Global hotkey registered (Cmd+Shift+V)")
    }

    private func handleHotkey(_ event: NSEvent) {
        let shiftPressed = event.modifierFlags.contains(.shift)
        let cmdPressed = event.modifierFlags.contains(.command)
        let vPressed = event.keyCode == 0x09 // V key

        if cmdPressed && shiftPressed && vPressed {
            DispatchQueue.main.async { [weak self] in
                self?.togglePanel()
            }
        }
    }

    // ── Toggle ──
    func togglePanel() {
        guard let panel = panel else { return }
        if panel.isVisible {
            hidePanel()
        } else {
            showPanel()
        }
    }

    func showPanel() {
        guard let panel = panel else { return }

        // Position below status bar icon
        if let button = statusItem.button {
            let buttonFrame = button.window?.convertToScreen(button.frame) ?? .zero
            let panelWidth = panel.frame.width
            let panelHeight = panel.frame.height
            let x = buttonFrame.midX - panelWidth / 2
            let y = buttonFrame.minY - panelHeight - 8
            panel.setFrameOrigin(NSPoint(x: max(x, 8), y: max(y, 8)))
        }

        panel.orderFrontRegardless()

        // Reload
        webView?.reload()

        // Listen for clicks outside panel
        setupClickOutsideMonitors()
    }

    func setupClickOutsideMonitors() {
        cleanupClickMonitors()

        // Global: clicks in OTHER apps
        clickOutsideMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) { [weak self] event in
            self?.hidePanel()
        }

        // Local: clicks in OUR app but NOT on the panel
        clickLocalMonitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) { [weak self] event in
            guard let self = self, let panel = self.panel, panel.isVisible else {
                return event
            }
            // If click is on the panel itself, don't close
            if event.window == panel {
                return event
            }
            // Click is on another window in our app (e.g., menu bar) - close panel
            self.hidePanel()
            return event
        }
    }

    func hidePanel() {
        panel?.orderOut(nil)
        cleanupClickMonitors()
    }

    func cleanupClickMonitors() {
        if let monitor = clickOutsideMonitor {
            NSEvent.removeMonitor(monitor)
            clickOutsideMonitor = nil
        }
        if let monitor = clickLocalMonitor {
            NSEvent.removeMonitor(monitor)
            clickLocalMonitor = nil
        }
    }

    func cleanupMonitors() {
        cleanupClickMonitors()
        if let monitor = hotkeyGlobalMonitor {
            NSEvent.removeMonitor(monitor)
            hotkeyGlobalMonitor = nil
        }
        if let monitor = hotkeyLocalMonitor {
            NSEvent.removeMonitor(monitor)
            hotkeyLocalMonitor = nil
        }
    }
}

// ── Main ──
@main
struct ClippyApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        Settings { EmptyView() }
    }
}
