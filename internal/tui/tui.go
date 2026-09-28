package tui

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/romboooo/release-radar/internal/store"
	"github.com/romboooo/release-radar/internal/tracker"
	"golang.org/x/sys/unix"
)

const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	dim     = "\x1b[2m"
	gray    = "\x1b[38;5;245m"
	green   = "\x1b[38;5;114m"
	red     = "\x1b[38;5;167m"
	yellow  = "\x1b[38;5;179m"
	cyan    = "\x1b[38;5;110m"
	inverse = "\x1b[7m"
)

type viewMode int

const (
	repositoriesView viewMode = iota
	checksView
	historyView
)

type inputMode int

const (
	browseMode inputMode = iota
	addMode
	deleteMode
)

type checkItem struct {
	Repo   string
	Source string
	Tag    string
	Status string
	Err    string
}

type app struct {
	service *tracker.Service
	in      *os.File
	out     io.Writer

	width  int
	height int

	view       viewMode
	mode       inputMode
	selected   int
	status     string
	statusGood bool
	input      string

	repos   []store.Repository
	history []store.HistoryRecord
	checks  []checkItem
}

func Run(ctx context.Context, service *tracker.Service) error {
	a := &app{
		service: service,
		in:      os.Stdin,
		out:     os.Stdout,
		status:  "ready",
	}
	return a.run(ctx)
}

func (a *app) run(ctx context.Context) error {
	if !isTerminal(a.in) {
		return errors.New("tui requires an interactive terminal")
	}

	restore, err := enableRawMode(a.in)
	if err != nil {
		return fmt.Errorf("enable terminal raw mode: %w", err)
	}
	defer restore()

	fmt.Fprint(a.out, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(a.out, "\x1b[?25h\x1b[?1049l\x1b[0m")

	a.refreshRepos(ctx)
	a.render()

	reader := bufio.NewReader(a.in)
	for {
		key, err := readKey(reader, int(a.in.Fd()))
		if err != nil {
			return err
		}

		done := a.handleKey(ctx, key)
		a.render()
		if done {
			return nil
		}
	}
}

func (a *app) handleKey(ctx context.Context, key string) bool {
	if a.mode != browseMode {
		return a.handleInputKey(ctx, key)
	}

	switch key {
	case "q", "ctrl-c":
		return true
	case "tab", "l", "right":
		a.view = (a.view + 1) % 3
		a.selected = 0
		a.refreshCurrent(ctx)
	case "h", "left":
		a.view = (a.view + 2) % 3
		a.selected = 0
		a.refreshCurrent(ctx)
	case "j", "down":
		a.move(1)
	case "k", "up":
		a.move(-1)
	case "r":
		a.refreshCurrent(ctx)
	case "a":
		a.mode = addMode
		a.input = ""
		a.status = "add repository as owner/repo"
		a.statusGood = true
	case "d":
		if a.view != repositoriesView || len(a.repos) == 0 {
			a.setError("select a tracked repository first")
			break
		}
		repo := a.repos[a.selected]
		a.mode = deleteMode
		a.input = repo.Owner + "/" + repo.Repo
		a.status = "press enter to delete, esc to cancel"
		a.statusGood = false
	case "c":
		a.runCheck(ctx)
	}

	return false
}

func (a *app) handleInputKey(ctx context.Context, key string) bool {
	switch key {
	case "esc":
		a.mode = browseMode
		a.input = ""
		a.setOK("cancelled")
	case "backspace":
		if len(a.input) > 0 {
			a.input = a.input[:len(a.input)-1]
		}
	case "enter":
		value := strings.TrimSpace(a.input)
		action := a.mode
		a.mode = browseMode
		a.input = ""
		switch a.modeAction(action, value, ctx) {
		case true:
			a.refreshRepos(ctx)
		}
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			a.input += key
		}
	}
	return false
}

func (a *app) modeAction(action inputMode, value string, ctx context.Context) bool {
	switch action {
	case browseMode:
		return false
	case addMode:
		owner, repo, ok := parseRepo(value)
		if !ok {
			a.setError("repository must have the form owner/repo")
			return false
		}
		if err := a.service.AddRepository(ctx, owner, repo); err != nil {
			a.setError(err.Error())
			return false
		}
		a.setOK("added " + owner + "/" + repo)
		return true
	case deleteMode:
		owner, repo, ok := parseRepo(value)
		if !ok {
			a.setError("repository must have the form owner/repo")
			return false
		}
		if err := a.service.Delete(ctx, owner, repo); err != nil {
			a.setError(err.Error())
			return false
		}
		a.setOK("deleted " + owner + "/" + repo)
		return true
	default:
		return false
	}
}

func (a *app) refreshCurrent(ctx context.Context) {
	switch a.view {
	case repositoriesView:
		a.refreshRepos(ctx)
	case checksView:
		if len(a.checks) == 0 {
			a.runCheck(ctx)
		}
	case historyView:
		a.refreshHistory(ctx)
	}
}

func (a *app) refreshRepos(ctx context.Context) {
	repos, err := a.service.ListRepositories(ctx)
	if err != nil {
		a.setError(err.Error())
		return
	}
	a.repos = repos
	a.clampSelection()
	a.setOK(fmt.Sprintf("%d repositories", len(repos)))
}

func (a *app) refreshHistory(ctx context.Context) {
	history, err := a.service.ListHistory(ctx)
	if err != nil {
		a.setError(err.Error())
		return
	}
	a.history = history
	a.clampSelection()
	a.setOK(fmt.Sprintf("%d history records", len(history)))
}

func (a *app) runCheck(ctx context.Context) {
	a.view = checksView
	a.status = "checking repositories..."
	a.statusGood = true
	a.render()

	results, err := a.service.Check(ctx)
	if err != nil {
		a.setError(err.Error())
		return
	}

	checks := make([]checkItem, 0, len(results))
	for _, result := range results {
		item := checkItem{
			Repo:   result.Repository.Owner + "/" + result.Repository.Repo,
			Source: result.Source,
		}
		if result.Err != nil {
			item.Status = "error"
			item.Err = result.Err.Error()
		} else {
			item.Status = "seen"
			if result.IsNew {
				item.Status = "new"
			}
			if result.Source == "tag" {
				item.Tag = result.Tag.Name
			} else {
				item.Tag = result.Release.TagName
			}
		}
		checks = append(checks, item)
	}
	a.checks = checks
	a.selected = 0
	a.setOK(fmt.Sprintf("checked %d repositories", len(checks)))
}

func (a *app) render() {
	a.width, a.height = terminalSize(a.in)
	if a.width < 64 {
		a.width = 64
	}
	if a.height < 18 {
		a.height = 18
	}

	var buf bytes.Buffer
	buf.WriteString("\x1b[H\x1b[2J")
	buf.WriteString(a.header())

	bodyHeight := a.height - 4
	sidebarWidth := 22
	mainWidth := a.width - sidebarWidth - 3

	sidebar := a.sidebar(bodyHeight, sidebarWidth)
	main := a.main(bodyHeight, mainWidth)

	for i := 0; i < bodyHeight; i++ {
		buf.WriteString(sidebar[i])
		buf.WriteString(gray + " | " + reset)
		buf.WriteString(main[i])
		buf.WriteString("\r\n")
	}

	buf.WriteString(a.footer())
	fmt.Fprint(a.out, buf.String())
}

func (a *app) header() string {
	title := " rradar "
	return bold + title + reset + gray + strings.Repeat("-", max(1, a.width-visibleLen(title))) + reset + "\r\n"
}

func (a *app) sidebar(height, width int) []string {
	rows := make([]string, height)
	items := []struct {
		mode  viewMode
		label string
		count int
	}{
		{repositoriesView, "repositories", len(a.repos)},
		{checksView, "checks", len(a.checks)},
		{historyView, "history", len(a.history)},
	}

	for i := range rows {
		rows[i] = pad("", width)
	}
	rows[0] = dim + pad("views", width) + reset
	for i, item := range items {
		label := fmt.Sprintf("%s %d", item.label, item.count)
		if a.view == item.mode {
			rows[i+2] = inverse + pad(" "+label, width) + reset
		} else {
			rows[i+2] = pad(" "+label, width)
		}
	}

	help := []string{
		"keys",
		"j/k move",
		"tab switch",
		"a add",
		"d delete",
		"c check",
		"r refresh",
		"q quit",
	}
	start := max(0, height-len(help)-1)
	for i, line := range help {
		if start+i >= height {
			break
		}
		style := gray
		if i == 0 {
			style = dim
		}
		rows[start+i] = style + pad(line, width) + reset
	}
	return rows
}

func (a *app) main(height, width int) []string {
	rows := make([]string, height)
	for i := range rows {
		rows[i] = pad("", width)
	}

	switch a.view {
	case repositoriesView:
		rows[0] = bold + pad("Tracked repositories", width) + reset
		if len(a.repos) == 0 {
			rows[2] = gray + pad("No repositories yet. Press a to add one.", width) + reset
			return rows
		}
		for i, repo := range visibleWindow(len(a.repos), a.selected, height-2) {
			line := repoLine(a.repos[repo])
			rows[i+2] = a.selectLine(repo, line, width)
		}
	case checksView:
		rows[0] = bold + pad("Latest check", width) + reset
		if len(a.checks) == 0 {
			rows[2] = gray + pad("Press c to check tracked repositories.", width) + reset
			return rows
		}
		for i, idx := range visibleWindow(len(a.checks), a.selected, height-2) {
			rows[i+2] = a.selectLine(idx, checkLine(a.checks[idx]), width)
		}
	case historyView:
		rows[0] = bold + pad("History", width) + reset
		if len(a.history) == 0 {
			rows[2] = gray + pad("No discovered versions yet.", width) + reset
			return rows
		}
		for i, idx := range visibleWindow(len(a.history), a.selected, height-2) {
			rows[i+2] = a.selectLine(idx, historyLine(a.history[idx]), width)
		}
	}
	return rows
}

func (a *app) footer() string {
	var left string
	if a.mode == addMode {
		left = "add: " + a.input
	} else if a.mode == deleteMode {
		left = "delete: " + a.input
	} else {
		left = a.status
	}

	color := red
	if a.statusGood {
		color = green
	}
	line := color + truncate(left, a.width) + reset
	return gray + strings.Repeat("-", a.width) + reset + "\r\n" + pad(line, a.width) + "\r\n"
}

func (a *app) selectLine(idx int, line string, width int) string {
	line = truncate(line, width)
	if idx == a.selected {
		return inverse + pad(" "+line, width) + reset
	}
	return pad(" "+line, width)
}

func repoLine(repo store.Repository) string {
	return repo.Owner + "/" + repo.Repo
}

func checkLine(item checkItem) string {
	if item.Err != "" {
		return fmt.Sprintf("%s  %s%s%s  %s", item.Repo, red, item.Status, reset, item.Err)
	}
	status := gray + "seen" + reset
	if item.Status == "new" {
		status = green + "new" + reset
	}
	return fmt.Sprintf("%s  %s  %s%s%s  %s", item.Repo, item.Tag, cyan, item.Source, reset, status)
}

func historyLine(record store.HistoryRecord) string {
	return fmt.Sprintf(
		"%s/%s  %s  %s%s%s  %s",
		record.Owner,
		record.Repo,
		record.TagName,
		yellow,
		record.Source,
		reset,
		firstNonEmpty(record.PublishedAt, record.DiscoveredAt),
	)
}

func (a *app) move(delta int) {
	count := a.itemCount()
	if count == 0 {
		return
	}
	a.selected += delta
	if a.selected < 0 {
		a.selected = count - 1
	}
	if a.selected >= count {
		a.selected = 0
	}
}

func (a *app) itemCount() int {
	switch a.view {
	case repositoriesView:
		return len(a.repos)
	case checksView:
		return len(a.checks)
	case historyView:
		return len(a.history)
	default:
		return 0
	}
}

func (a *app) clampSelection() {
	count := a.itemCount()
	if count == 0 {
		a.selected = 0
		return
	}
	if a.selected >= count {
		a.selected = count - 1
	}
}

func (a *app) setOK(message string) {
	a.status = message
	a.statusGood = true
}

func (a *app) setError(message string) {
	a.status = message
	a.statusGood = false
}

func parseRepo(value string) (string, string, bool) {
	owner, repo, found := strings.Cut(value, "/")
	return owner, repo, found && owner != "" && repo != "" && !strings.Contains(repo, "/")
}

func readKey(reader *bufio.Reader, fd int) (string, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return "", err
	}

	switch b {
	case 3:
		return "ctrl-c", nil
	case 9:
		return "tab", nil
	case 13, 10:
		return "enter", nil
	case 27:
		if reader.Buffered() == 0 && !hasInput(fd, 20*time.Millisecond) {
			return "esc", nil
		}
		next, err := reader.ReadByte()
		if err != nil || next != '[' {
			return "esc", nil
		}
		if reader.Buffered() == 0 && !hasInput(fd, 20*time.Millisecond) {
			return "esc", nil
		}
		code, err := reader.ReadByte()
		if err != nil {
			return "esc", nil
		}
		switch code {
		case 'A':
			return "up", nil
		case 'B':
			return "down", nil
		case 'C':
			return "right", nil
		case 'D':
			return "left", nil
		default:
			return "esc", nil
		}
	case 127, 8:
		return "backspace", nil
	default:
		return string([]byte{b}), nil
	}
}

func hasInput(fd int, timeout time.Duration) bool {
	pollFDs := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	n, err := unix.Poll(pollFDs, int(timeout/time.Millisecond))
	return err == nil && n > 0 && pollFDs[0].Revents&unix.POLLIN != 0
}

func enableRawMode(file *os.File) (func(), error) {
	fd := int(file.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}
	newState := *oldState
	newState.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	newState.Oflag &^= unix.OPOST
	newState.Cflag |= unix.CS8
	newState.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, err
	}

	return func() {
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, oldState)
	}, nil
}

func isTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS)
	return err == nil
}

func terminalSize(file *os.File) (int, int) {
	size, err := unix.IoctlGetWinsize(int(file.Fd()), unix.TIOCGWINSZ)
	if err != nil || size.Col == 0 || size.Row == 0 {
		return 100, 32
	}
	return int(size.Col), int(size.Row)
}

func visibleWindow(total, selected, height int) []int {
	if height <= 0 || total == 0 {
		return nil
	}
	start := 0
	if selected >= height {
		start = selected - height + 1
	}
	end := min(total, start+height)
	indexes := make([]int, 0, end-start)
	for i := start; i < end; i++ {
		indexes = append(indexes, i)
	}
	return indexes
}

func pad(value string, width int) string {
	length := visibleLen(value)
	if length >= width {
		return truncate(value, width)
	}
	return value + strings.Repeat(" ", width-length)
}

func truncate(value string, width int) string {
	if visibleLen(value) <= width {
		return value
	}
	var out strings.Builder
	printed := 0
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		out.WriteByte(ch)
		if inEscape {
			if ch == 'm' {
				inEscape = false
			}
			continue
		}
		if ch == '\x1b' {
			inEscape = true
			continue
		}
		printed++
		if printed >= max(0, width-1) {
			break
		}
	}
	if width > 0 {
		out.WriteString("~")
	}
	return out.String()
}

func visibleLen(value string) int {
	length := 0
	inEscape := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inEscape {
			if ch == 'm' {
				inEscape = false
			}
			continue
		}
		if ch == '\x1b' {
			inEscape = true
			continue
		}
		length++
	}
	return length
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			if parsed, err := time.Parse(time.RFC3339, value); err == nil {
				return parsed.Format("2006-01-02")
			}
			return value
		}
	}
	return ""
}
