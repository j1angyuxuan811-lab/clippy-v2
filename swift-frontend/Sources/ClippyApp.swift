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

    private let backendURL: URL = {
        let appBundleURL = Bundle.main.bundleURL
        return appBundleURL.appendingPathComponent("Contents/Resources/clippy-server")
    }()

    private var uiDir: URL {
        if let resourcesPath = Bundle.main.resourcePath {
            return URL(fileURLWithPath: resourcesPath)
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
    }

    func applicationWillTerminate(_ notification: Notification) {
        stopBackend()
        cleanupMonitors()
    }

    func applicationDidResignActive(_ notification: Notification) {
        // Don't auto-hide when clicking outside (we handle it manually)
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
            styleMask: [.titled, .fullSizeContentView, .nonactivatingPanel],
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
