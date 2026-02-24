import AppKit
import Foundation

struct Choice {
    let label: String
    let command: String
}

struct Config {
    let title: String
    let message: String
    let identifier: String
    let timeoutSeconds: Int
    let interactionLockFile: String
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
    let identifier = value("--identifier") ?? ""
    let interactionLockFile = value("--interaction-lock-file") ?? ""

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
        identifier: identifier,
        timeoutSeconds: timeoutSeconds,
        interactionLockFile: interactionLockFile,
        choices: choices
    )
}

private func clearInteractionLock(_ path: String) {
    let trimmed = path.trimmingCharacters(in: .whitespacesAndNewlines)
    guard !trimmed.isEmpty else {
        return
    }
    _ = try? FileManager.default.removeItem(atPath: trimmed)
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

enum ChoiceIntent {
    case primary
    case destructive
    case neutral
    case secondary
}

private func choiceIntent(for label: String, index: Int, total: Int) -> ChoiceIntent {
    var normalized = label.lowercased().trimmingCharacters(in: .whitespacesAndNewlines)
    normalized = normalized.replacingOccurrences(of: " ", with: "")
    normalized = normalized.replacingOccurrences(of: "-", with: "")
    normalized = normalized.replacingOccurrences(of: "_", with: "")

    switch normalized {
    case "open", "focus", "show", "view":
        return .neutral
    case "approve", "approved", "allow", "yes", "y", "ok":
        return .primary
    case "reject", "denied", "deny", "no", "n", "cancel":
        return .destructive
    default:
        break
    }

    if total == 2 {
        return index == 0 ? .primary : .destructive
    }
    if index == 0 {
        return .primary
    }
    return .secondary
}

private func shortenedIdentifier(_ raw: String) -> String {
    let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    if trimmed.count <= 22 {
        return trimmed
    }

    let head = trimmed.prefix(10)
    let tail = trimmed.suffix(8)
    return "\(head)...\(tail)"
}

private func chunkChoices(_ choices: [Choice], columns: Int) -> [[Choice]] {
    guard columns > 0 else {
        return [choices]
    }

    var rows: [[Choice]] = []
    var index = 0
    while index < choices.count {
        let end = min(index + columns, choices.count)
        rows.append(Array(choices[index..<end]))
        index = end
    }
    return rows
}

private func columnsPerRow(choiceCount: Int) -> Int {
    switch choiceCount {
    case ..<1:
        return 1
    case 1...3:
        return choiceCount
    case 4...6:
        return 3
    default:
        return 4
    }
}

private struct ButtonPalette {
    let base: NSColor
    let hover: NSColor
    let border: NSColor
    let text: NSColor
}

private func buttonPalette(for intent: ChoiceIntent) -> ButtonPalette {
    switch intent {
    case .primary:
        return ButtonPalette(
            base: NSColor.systemBlue.withAlphaComponent(0.85),
            hover: NSColor.systemBlue.withAlphaComponent(0.98),
            border: NSColor.systemBlue.withAlphaComponent(1.0),
            text: NSColor.white
        )
    case .destructive:
        return ButtonPalette(
            base: NSColor.systemRed.withAlphaComponent(0.83),
            hover: NSColor.systemRed.withAlphaComponent(0.96),
            border: NSColor.systemRed.withAlphaComponent(1.0),
            text: NSColor.white
        )
    case .neutral:
        return ButtonPalette(
            base: NSColor.white.withAlphaComponent(0.16),
            hover: NSColor.white.withAlphaComponent(0.24),
            border: NSColor.white.withAlphaComponent(0.32),
            text: NSColor.labelColor
        )
    case .secondary:
        return ButtonPalette(
            base: NSColor.controlAccentColor.withAlphaComponent(0.35),
            hover: NSColor.controlAccentColor.withAlphaComponent(0.55),
            border: NSColor.controlAccentColor.withAlphaComponent(0.7),
            text: NSColor.labelColor
        )
    }
}

final class StyledActionButton: NSButton {
    private let palette: ButtonPalette
    private var tracking: NSTrackingArea?

    init(title: String, intent: ChoiceIntent, index: Int, target: AnyObject?, action: Selector?) {
        self.palette = buttonPalette(for: intent)
        super.init(frame: .zero)

        self.target = target
        self.action = action
        self.title = title
        self.tag = index
        self.isBordered = false
        self.bezelStyle = .regularSquare
        self.setButtonType(.momentaryPushIn)
        self.font = NSFont.systemFont(ofSize: 12, weight: .semibold)
        self.focusRingType = .none
        self.wantsLayer = true
        self.layer?.cornerRadius = 8
        self.layer?.borderWidth = 1
        self.layer?.masksToBounds = true

        if index == 0 {
            self.keyEquivalent = "\r"
            self.keyEquivalentModifierMask = []
        } else if index < 9 {
            self.keyEquivalent = String(index + 1)
            self.keyEquivalentModifierMask = []
        }

        self.attributedTitle = NSAttributedString(
            string: title,
            attributes: [
                .font: NSFont.systemFont(ofSize: 12, weight: .semibold),
                .foregroundColor: palette.text
            ]
        )

        apply(baseColor: palette.base)
        self.layer?.borderColor = palette.border.cgColor
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) {
        nil
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()

        if let tracking {
            removeTrackingArea(tracking)
        }

        let options: NSTrackingArea.Options = [.activeInKeyWindow, .mouseEnteredAndExited, .inVisibleRect]
        let tracking = NSTrackingArea(rect: bounds, options: options, owner: self, userInfo: nil)
        addTrackingArea(tracking)
        self.tracking = tracking
    }

    override func mouseEntered(with event: NSEvent) {
        super.mouseEntered(with: event)
        apply(baseColor: palette.hover)
    }

    override func mouseExited(with event: NSEvent) {
        super.mouseExited(with: event)
        apply(baseColor: palette.base)
    }

    override func mouseDown(with event: NSEvent) {
        apply(baseColor: palette.hover.blended(withFraction: 0.2, of: NSColor.black) ?? palette.hover)
        super.mouseDown(with: event)
        apply(baseColor: palette.base)
    }

    private func apply(baseColor: NSColor) {
        layer?.backgroundColor = baseColor.cgColor
    }
}

final class PopupPanel: NSPanel {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }
}

final class PopupController: NSObject {
    private let config: Config
    private var panel: PopupPanel?
    private var timeoutTimer: Timer?
    private var progressTimer: Timer?
    private var progressFill: NSView?
    private var progressTrackWidth: CGFloat = 0
    private var openedAt = Date()
    private var isClosing = false
    private let fixedWidth: CGFloat = 392
    private let fixedHeight: CGFloat = 186
    private let horizontalPadding: CGFloat = 14
    private let messageAreaHeight: CGFloat = 40
    private let messageMaxLines: Int = 2

    init(config: Config) {
        self.config = config
    }

    deinit {
        releaseInteractionLock()
    }

    func releaseInteractionLock() {
        clearInteractionLock(config.interactionLockFile)
    }

    func show() {
        let columns = columnsPerRow(choiceCount: config.choices.count)
        let width = fixedWidth
        let panelHeight = fixedHeight
        let rows = chunkChoices(config.choices, columns: columns)

        let visible = NSScreen.main?.visibleFrame ?? NSScreen.screens.first?.visibleFrame ?? NSRect(x: 0, y: 0, width: 1200, height: 800)
        let x = visible.maxX - width - 18
        let y = visible.minY + 22
        let finalFrame = NSRect(x: x, y: y, width: width, height: panelHeight)
        let startFrame = NSRect(x: x, y: y - 14, width: width, height: panelHeight)

        let panel = PopupPanel(
            contentRect: startFrame,
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

        let root = NSVisualEffectView(frame: NSRect(origin: .zero, size: finalFrame.size))
        root.autoresizingMask = [.width, .height]
        root.blendingMode = .withinWindow
        root.state = .active
        root.material = .popover
        root.wantsLayer = true
        root.layer?.cornerRadius = 16
        root.layer?.borderWidth = 1
        root.layer?.borderColor = NSColor.white.withAlphaComponent(0.18).cgColor
        root.layer?.masksToBounds = true
        panel.contentView = root

        let tint = NSView(frame: root.bounds)
        tint.autoresizingMask = [.width, .height]
        tint.wantsLayer = true
        tint.layer?.backgroundColor = NSColor.controlAccentColor.withAlphaComponent(0.08).cgColor
        root.addSubview(tint)

        let accentBar = NSView(frame: NSRect(x: 0, y: 0, width: 4, height: panelHeight))
        accentBar.wantsLayer = true
        accentBar.layer?.backgroundColor = NSColor.controlAccentColor.withAlphaComponent(0.85).cgColor
        root.addSubview(accentBar)

        let headerHeight: CGFloat = 30
        let headerY = panelHeight - 14 - headerHeight

        let iconBack = NSView(frame: NSRect(x: horizontalPadding, y: headerY + 7, width: 18, height: 18))
        iconBack.wantsLayer = true
        iconBack.layer?.cornerRadius = 9
        iconBack.layer?.backgroundColor = NSColor.controlAccentColor.withAlphaComponent(0.2).cgColor
        root.addSubview(iconBack)

        let iconView = NSImageView(frame: NSRect(x: horizontalPadding + 2, y: headerY + 9, width: 14, height: 14))
        if #available(macOS 11.0, *) {
            iconView.image = NSImage(systemSymbolName: "bolt.fill", accessibilityDescription: nil)
            iconView.symbolConfiguration = NSImage.SymbolConfiguration(pointSize: 10, weight: .medium)
        }
        iconView.contentTintColor = NSColor.controlAccentColor
        root.addSubview(iconView)

        let titleLabel = NSTextField(labelWithString: config.title)
        titleLabel.frame = NSRect(x: horizontalPadding + 24, y: headerY + 11, width: width - 82, height: 16)
        titleLabel.font = NSFont.systemFont(ofSize: 12, weight: .semibold)
        titleLabel.textColor = .labelColor
        root.addSubview(titleLabel)

        var meta = "codex-notify"
        if !config.identifier.isEmpty {
            meta += "  •  \(shortenedIdentifier(config.identifier))"
        }
        let metaLabel = NSTextField(labelWithString: meta)
        metaLabel.frame = NSRect(x: horizontalPadding + 24, y: headerY - 1, width: width - 82, height: 12)
        metaLabel.font = NSFont.systemFont(ofSize: 10, weight: .medium)
        metaLabel.textColor = .tertiaryLabelColor
        root.addSubview(metaLabel)

        let closeButton = NSButton(title: "×", target: self, action: #selector(closePopup))
        closeButton.isBordered = false
        closeButton.frame = NSRect(x: width - 28, y: headerY + 9, width: 14, height: 14)
        closeButton.font = NSFont.systemFont(ofSize: 12, weight: .semibold)
        closeButton.contentTintColor = .tertiaryLabelColor
        root.addSubview(closeButton)

        let messageWidth = width - (horizontalPadding * 2)
        let messageY = headerY - 8 - messageAreaHeight
        let messageLabel = NSTextField(wrappingLabelWithString: config.message)
        messageLabel.frame = NSRect(x: horizontalPadding, y: messageY, width: messageWidth, height: messageAreaHeight)
        messageLabel.font = NSFont.systemFont(ofSize: 12, weight: .regular)
        messageLabel.textColor = .secondaryLabelColor
        messageLabel.maximumNumberOfLines = messageMaxLines
        messageLabel.alignment = .left
        if let messageCell = messageLabel.cell as? NSTextFieldCell {
            messageCell.wraps = true
            messageCell.lineBreakMode = .byTruncatingTail
            messageCell.usesSingleLineMode = false
        }
        root.addSubview(messageLabel)

        let progressHeight: CGFloat = 3
        let progressY = messageY - 9 - progressHeight
        let progressTrack = NSView(frame: NSRect(x: horizontalPadding, y: progressY, width: width - (horizontalPadding * 2), height: progressHeight))
        progressTrack.wantsLayer = true
        progressTrack.layer?.cornerRadius = progressHeight / 2
        progressTrack.layer?.masksToBounds = true
        progressTrack.layer?.backgroundColor = NSColor.white.withAlphaComponent(0.14).cgColor

        let progressFill = NSView(frame: progressTrack.bounds)
        progressFill.autoresizingMask = [.height]
        progressFill.wantsLayer = true
        progressFill.layer?.cornerRadius = progressHeight / 2
        progressFill.layer?.backgroundColor = NSColor.controlAccentColor.withAlphaComponent(0.95).cgColor
        progressTrack.addSubview(progressFill)
        root.addSubview(progressTrack)
        self.progressFill = progressFill
        self.progressTrackWidth = progressTrack.bounds.width

        let readMoreButton = NSButton(title: "Read more", target: self, action: #selector(showReadMore))
        readMoreButton.isBordered = false
        readMoreButton.font = NSFont.systemFont(ofSize: 10, weight: .semibold)
        readMoreButton.contentTintColor = NSColor.controlAccentColor
        readMoreButton.frame = NSRect(x: width - horizontalPadding - 66, y: progressY + progressHeight + 1, width: 66, height: 14)
        readMoreButton.alignment = .right
        root.addSubview(readMoreButton)

        let availableButtonsTop = progressY - 8
        let availableButtonsBottom: CGFloat = 12
        let availableButtonsHeight = max(36, availableButtonsTop - availableButtonsBottom)
        let desiredRowHeight: CGFloat = 28
        var rowHeight: CGFloat = desiredRowHeight
        var rowSpacing: CGFloat = 6
        if rows.count > 1 {
            let maxRowHeight = (availableButtonsHeight - (CGFloat(rows.count - 1) * rowSpacing)) / CGFloat(rows.count)
            rowHeight = max(18, min(desiredRowHeight, floor(maxRowHeight)))
            if rowHeight < desiredRowHeight {
                rowSpacing = 4
            }
        }
        let buttonsHeight = CGFloat(rows.count) * rowHeight + CGFloat(max(0, rows.count - 1)) * rowSpacing
        let buttonsY = availableButtonsBottom + max(0, (availableButtonsHeight - buttonsHeight) / 2)
        var nextRowTop = buttonsY + buttonsHeight - rowHeight
        var globalIndex = 0
        for row in rows {
            let rowStack = NSStackView(frame: NSRect(x: horizontalPadding, y: nextRowTop, width: width - (horizontalPadding * 2), height: rowHeight))
            rowStack.orientation = .horizontal
            rowStack.alignment = .centerY
            rowStack.distribution = .fillEqually
            rowStack.spacing = rowSpacing
            for choice in row {
                let intent = choiceIntent(for: choice.label, index: globalIndex, total: config.choices.count)
                let button = StyledActionButton(
                    title: choice.label,
                    intent: intent,
                    index: globalIndex,
                    target: self,
                    action: #selector(choiceClicked(_:))
                )
                rowStack.addArrangedSubview(button)
                globalIndex += 1
            }
            root.addSubview(rowStack)
            nextRowTop -= rowHeight + rowSpacing
        }

        self.panel = panel
        openedAt = Date()
        panel.alphaValue = 0
        panel.orderFrontRegardless()
        NSAnimationContext.runAnimationGroup { context in
            context.duration = 0.17
            context.timingFunction = CAMediaTimingFunction(name: .easeOut)
            panel.animator().alphaValue = 1
            panel.animator().setFrame(finalFrame, display: true)
        }

        timeoutTimer = Timer.scheduledTimer(
            timeInterval: TimeInterval(config.timeoutSeconds),
            target: self,
            selector: #selector(closePopup),
            userInfo: nil,
            repeats: false
        )
        progressTimer = Timer.scheduledTimer(
            timeInterval: 0.05,
            target: self,
            selector: #selector(updateProgress),
            userInfo: nil,
            repeats: true
        )
    }

    private func textHeight(_ text: String, width: CGFloat) -> CGFloat {
        let paragraph = NSMutableParagraphStyle()
        paragraph.lineSpacing = 2
        let attr = NSAttributedString(
            string: text,
            attributes: [
                .font: NSFont.systemFont(ofSize: 13, weight: .regular),
                .paragraphStyle: paragraph
            ]
        )
        let rect = attr.boundingRect(with: NSSize(width: width, height: 400), options: [.usesLineFragmentOrigin, .usesFontLeading])
        return max(28, ceil(rect.height))
    }

    @objc private func showReadMore() {
        let alert = NSAlert()
        alert.alertStyle = .informational
        alert.messageText = config.title
        alert.informativeText = config.message
        alert.addButton(withTitle: "Close")
        if let panel {
            alert.beginSheetModal(for: panel)
        } else {
            _ = alert.runModal()
        }
    }

    @objc private func updateProgress() {
        guard let fill = progressFill else {
            return
        }
        let elapsed = Date().timeIntervalSince(openedAt)
        let ratio = max(0, min(1, 1 - (elapsed / Double(config.timeoutSeconds))))
        var frame = fill.frame
        frame.size.width = progressTrackWidth * CGFloat(ratio)
        fill.frame = frame
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
        if isClosing {
            return
        }
        isClosing = true

        timeoutTimer?.invalidate()
        timeoutTimer = nil
        progressTimer?.invalidate()
        progressTimer = nil
        releaseInteractionLock()

        guard let panel else {
            NSApp.terminate(nil)
            return
        }

        var fadedFrame = panel.frame
        fadedFrame.origin.y -= 10
        NSAnimationContext.runAnimationGroup({ context in
            context.duration = 0.12
            context.timingFunction = CAMediaTimingFunction(name: .easeIn)
            panel.animator().alphaValue = 0
            panel.animator().setFrame(fadedFrame, display: true)
        }, completionHandler: {
            panel.orderOut(nil)
            self.panel = nil
            NSApp.terminate(nil)
        })
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

    func applicationWillTerminate(_ notification: Notification) {
        controller.releaseInteractionLock()
    }
}

let config = parseArgs(CommandLine.arguments)
let app = NSApplication.shared
app.setActivationPolicy(.accessory)
let controller = PopupController(config: config)
let delegate = AppDelegate(controller: controller)
app.delegate = delegate
app.run()
