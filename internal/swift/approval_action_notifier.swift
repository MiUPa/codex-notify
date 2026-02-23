import AppKit
import Foundation

struct Choice {
    let label: String
    let command: String
}

struct Config {
    let title: String
    let message: String
    let timeoutSeconds: Int
    let choices: [Choice]
}

private func parseArgs(_ args: [String]) -> Config {
    func value(_ key: String) -> String? {
        guard let idx = args.firstIndex(of: key), idx + 1 < args.count else {
            return nil
        }
        return args[idx + 1]
    }

    func values(_ key: String) -> [String] {
        var out: [String] = []
        var idx = 0
        while idx < args.count {
            if args[idx] == key && idx + 1 < args.count {
                out.append(args[idx + 1])
                idx += 2
                continue
            }
            idx += 1
        }
        return out
    }

    let title = value("--title") ?? "Codex: Approval Requested"
    let message = value("--message") ?? "承認待ちです。"

    let timeoutRaw = value("--timeout-seconds") ?? "45"
    let timeoutParsed = Int(timeoutRaw) ?? 45
    let timeoutSeconds = max(5, min(300, timeoutParsed))

    let labels = values("--choice-label")
    let commands = values("--choice-cmd")
    let count = min(labels.count, commands.count)

    var choices: [Choice] = []
    if count > 0 {
        for i in 0..<count {
            let label = labels[i].trimmingCharacters(in: .whitespacesAndNewlines)
            let command = commands[i].trimmingCharacters(in: .whitespacesAndNewlines)
            if label.isEmpty || command.isEmpty {
                continue
            }
            choices.append(Choice(label: label, command: command))
        }
    }

    if choices.isEmpty {
        choices = [
            Choice(label: "Open", command: ""),
            Choice(label: "Approve", command: ""),
            Choice(label: "Reject", command: "")
        ]
    }

    return Config(
        title: title,
        message: message,
        timeoutSeconds: timeoutSeconds,
        choices: choices
    )
}

private func runShell(_ command: String) {
    guard !command.isEmpty else {
        return
    }

    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/bin/zsh")
    process.arguments = ["-lc", command]

    if let nullOut = FileHandle(forWritingAtPath: "/dev/null") {
        process.standardOutput = nullOut
        process.standardError = nullOut
    }

    do {
        try process.run()
        process.waitUntilExit()
    } catch {
        fputs("failed to run action command: \(error)\n", stderr)
    }
}

final class PopupController: NSObject {
    private let config: Config
    private var panel: NSPanel?
    private var timeoutTimer: Timer?

    init(config: Config) {
        self.config = config
    }

    func show() {
        let width: CGFloat = max(360, min(720, CGFloat(220 + config.message.count * 2)))
        let messageHeight = textHeight(config.message, width: width - 36)
        let panelHeight: CGFloat = 70 + messageHeight + 48

        let visible = NSScreen.main?.visibleFrame ?? NSScreen.screens.first?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1200, height: 800)
        let x = visible.maxX - width - 18
        let y = visible.minY + 22
        let frame = NSRect(x: x, y: y, width: width, height: panelHeight)

        let panel = NSPanel(
            contentRect: frame,
            styleMask: [.nonactivatingPanel, .fullSizeContentView],
            backing: .buffered,
            defer: false
        )
        panel.level = .floating
        panel.backgroundColor = .clear
        panel.isOpaque = false
        panel.hasShadow = true
        panel.titleVisibility = .hidden
        panel.titlebarAppearsTransparent = true
        panel.hidesOnDeactivate = false
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient]

        let root = NSVisualEffectView(frame: NSRect(origin: .zero, size: frame.size))
        root.autoresizingMask = [.width, .height]
        root.blendingMode = .withinWindow
        root.state = .active
        root.material = .hudWindow
        root.wantsLayer = true
        root.layer?.cornerRadius = 12
        root.layer?.masksToBounds = true
        panel.contentView = root

        let titleLabel = NSTextField(labelWithString: config.title)
        titleLabel.frame = NSRect(x: 14, y: panelHeight - 34, width: width - 54, height: 20)
        titleLabel.font = NSFont.systemFont(ofSize: 14, weight: .semibold)
        titleLabel.textColor = .labelColor
        root.addSubview(titleLabel)

        let closeButton = NSButton(title: "×", target: self, action: #selector(closePopup))
        closeButton.isBordered = false
        closeButton.frame = NSRect(x: width - 34, y: panelHeight - 36, width: 20, height: 20)
        closeButton.font = NSFont.systemFont(ofSize: 16, weight: .bold)
        closeButton.contentTintColor = .secondaryLabelColor
        root.addSubview(closeButton)

        let messageLabel = NSTextField(wrappingLabelWithString: config.message)
        messageLabel.frame = NSRect(x: 14, y: 54, width: width - 28, height: messageHeight)
        messageLabel.font = NSFont.systemFont(ofSize: 13, weight: .regular)
        messageLabel.textColor = .secondaryLabelColor
        messageLabel.maximumNumberOfLines = 5
        root.addSubview(messageLabel)

        let stack = NSStackView(frame: NSRect(x: 14, y: 14, width: width - 28, height: 30))
        stack.orientation = .horizontal
        stack.alignment = .centerY
        stack.distribution = .fillEqually
        stack.spacing = 8
        for (idx, choice) in config.choices.enumerated() {
            let button = NSButton(title: choice.label, target: self, action: #selector(choiceClicked(_:)))
            button.tag = idx
            button.bezelStyle = .rounded
            stack.addArrangedSubview(button)
        }
        root.addSubview(stack)

        self.panel = panel
        panel.orderFrontRegardless()

        timeoutTimer = Timer.scheduledTimer(timeInterval: TimeInterval(config.timeoutSeconds), target: self, selector: #selector(closePopup), userInfo: nil, repeats: false)
    }

    private func textHeight(_ text: String, width: CGFloat) -> CGFloat {
        let attr = NSAttributedString(string: text, attributes: [.font: NSFont.systemFont(ofSize: 13)])
        let rect = attr.boundingRect(with: NSSize(width: width, height: 400), options: [.usesLineFragmentOrigin, .usesFontLeading])
        return max(20, ceil(rect.height))
    }

    @objc private func choiceClicked(_ sender: NSButton) {
        let idx = sender.tag
        guard idx >= 0, idx < config.choices.count else {
            closePopup()
            return
        }
        runShell(config.choices[idx].command)
        closePopup()
    }

    @objc private func closePopup() {
        timeoutTimer?.invalidate()
        timeoutTimer = nil
        panel?.orderOut(nil)
        panel = nil
        NSApp.terminate(nil)
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    private let controller: PopupController

    init(controller: PopupController) {
        self.controller = controller
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        controller.show()
    }
}

let config = parseArgs(CommandLine.arguments)
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let controller = PopupController(config: config)
let delegate = AppDelegate(controller: controller)
app.delegate = delegate
app.run()
