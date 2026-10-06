package formatter

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

type Console interface {
	Frame() *frame.Frame
	MaxWidth() int
	MinWidth() int
	Palette() *ansi.Palette
	Glyphs() *Glyphs
	Writer() io.Writer
}

// OnelineFunc formats a bookmark in a single line with the given colorscheme.
//
//	ID URL  @~*?  #go #tools
func OnelineFunc(c Console, b *bookmark.Bookmark) string {
	const (
		idWidth   = 4 // IDs up to 9999
		tagsWidth = 24
		gaps      = 2 // spaces between id|cell and cell|tags
		minURLLen = 10
	)

	p := c.Palette()

	// id (right-aligned, padded before coloring)
	idStr := strconv.Itoa(b.ID)
	id := strings.Repeat(" ", max(idWidth-txt.StringWidth(idStr), 0)) +
		p.BrightYellow.Wrap(idStr, p.Bold)

	// url + flags share one cell
	cellWidth := max(c.MaxWidth()-idWidth-tagsWidth-gaps, minURLLen)

	flags := formatColorFlags(p, c.Glyphs(), b)
	urlWidth := cellWidth

	if fw := txt.VisibleWidth(flags); fw > 0 {
		urlWidth = max(cellWidth-fw-1, minURLLen) // 1 = space before flags
	}

	u := p.Dim.Sprint(txt.Shorten(b.URL, urlWidth))

	cell := u
	if txt.VisibleWidth(flags) > 0 {
		cell += " " + flags
	}

	// tags (last column: truncated, not padded)
	tags := txt.Shorten(txt.TagsWith(b.Tags, c.Glyphs().Sep), tagsWidth)
	tags = p.Blue.Wrap(tags, p.Italic)

	var sb strings.Builder
	sb.Grow(c.MaxWidth() + 40)
	sb.WriteString(id)
	sb.WriteByte(' ')
	sb.WriteString(txt.PadRight(cell, cellWidth))
	sb.WriteByte(' ')
	sb.WriteString(tags)

	return sb.String()
}

// BriefFunc formats a bookmark as a simple, clean list item.
//
//	┃ ID Title (domain) #go #tools.
func BriefFunc(c Console, b *bookmark.Bookmark) string {
	p, w := c.Palette(), c.MaxWidth()

	const (
		bulletWidth = 1
		idMaxWidth  = 4
		spacing     = 3 // spaces between segments
		tagsBudget  = 40
	)

	idStr := strconv.Itoa(b.ID)

	domainPlain := ""
	if pu, err := url.Parse(b.URL); err == nil && pu.Host != "" {
		domainPlain = fmt.Sprintf(" (%s)", pu.Host)
	}
	domainWidth := txt.StringWidth(domainPlain)

	tagsPlain := ""
	if b.Tags != "" {
		tagsPlain = txt.TagsWith(b.Tags, c.Glyphs().Sep)
	}

	// width = total - (bullet + id + domain + tags + 3 spaces)
	overhead := bulletWidth + idMaxWidth + domainWidth + tagsBudget + spacing
	maxTitleWidth := max(w-overhead, 1)

	rawTitle := strings.ReplaceAll(b.Title, "\n", " ")
	if rawTitle == "" {
		rawTitle = b.URL
	}
	truncatedTitle := txt.Shorten(rawTitle, maxTitleWidth)
	// ensure the title block always occupies exactly maxtitlewidth on screen
	paddedTitle := txt.FillRight(truncatedTitle, maxTitleWidth)

	// bullet
	g := c.Glyphs().HeavyVertical
	bulletColored := p.Normal.Sprint(g)
	switch {
	case b.Favorite:
		bulletColored = p.Yellow.Sprint(g)
	case b.HTTPStatusCode >= 400:
		bulletColored = p.Red.Sprint(g)
	case b.Notes != "":
		bulletColored = p.Cyan.Sprint(g)
	}

	titleColored := p.Normal.Sprint(paddedTitle)
	if b.Title == "" {
		titleColored = p.Dim.Sprint(paddedTitle)
	}
	domainColored := p.Dim.Sprint(domainPlain)

	tagsColored := p.Blue.Wrap(tagsPlain, p.Italic)

	return fmt.Sprintf(
		"%s %s %s%s  %s",
		bulletColored,
		p.Dim.Sprintf("%-*s", idMaxWidth, idStr),
		titleColored,
		domainColored,
		tagsColored,
	)
}

// MultilineFunc formats a bookmark for fzf with max width.
func MultilineFunc(c Console, b *bookmark.Bookmark) string {
	p, w, g := c.Palette(), c.MaxWidth(), c.Glyphs()

	var sb strings.Builder
	sb.WriteString(p.BrightYellow.With(p.Bold).Sprint(b.ID))
	sb.WriteString(txt.NBSP)
	sb.WriteString(txt.URLBreadCrumbsColor(p, b.URL, g.Pointer, w))
	sb.WriteByte('\n')

	if b.Title != "" {
		title := strings.ReplaceAll(b.Title, "\n", " ")
		sb.WriteString(p.Cyan.Sprint(txt.Shorten(title, w)))
		sb.WriteByte('\n')
	}

	sb.WriteString(p.BrightWhite.Wrap(txt.TagsWith(b.Tags, g.Sep), p.Italic))
	sb.WriteByte('\n')

	return sb.String()
}

// FrameFunc formats a bookmark with a frame.
//
//   - ID [FLAGS] URL
//   - Title
//   - Desc
//   - Tags
func FrameFunc(c Console, b *bookmark.Bookmark) string {
	w, p, f := c.MaxWidth(), c.Palette(), c.Frame()

	// initial border adjustment
	borders := f.Borders()
	w -= len(borders.Row)

	idStr := strconv.Itoa(b.ID)
	// calculate visual width of id
	usedWidth := txt.StringWidth(idStr)

	idColor := p.BrightYellow.With(p.Bold).Sprint(idStr)
	header := []string{idColor}

	// prepare flags (if any) and accumulate width
	if flags := formatFlags(c.Glyphs(), b); flags != "" {
		flags = strings.TrimSpace(flags)
		// " [" + flags + "]"
		flagRaw := "[" + flags + "]"
		header = append(header, p.Dim.Sprint(flagRaw))

		// add flag width + 1 (for the space strings.join will add)
		usedWidth += txt.StringWidth(flagRaw) + 1
	}

	// calculate space for url
	// we subtract 'usedwidth' and 1 extra for the final space before the url
	urlWidth := w - usedWidth - 1

	header = append(header, txt.URLBreadCrumbsColor(p, b.URL, c.Glyphs().Pointer, urlWidth))
	f.Midln(strings.Join(header, " "))

	if b.Title != "" {
		titleSplit := txt.SplitIntoChunks(b.Title, w)
		f.Midln(p.StyleAll(titleSplit, p.BrightCyan)...)
	}

	if b.Desc != "" {
		descSplit := txt.SplitIntoChunks(b.Desc, w)
		f.Midln(p.StyleAll(descSplit, p.Dim)...)
	}

	return f.Footerln(txt.TagsWithColorPound(p, b.Tags)).
		StringReset()
}

func ParametersFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	const (
		idPadding      = 3
		idWithColor    = 4 // visible width for IDS up to 9999
		defaultTagsLen = 24
		minTagsLen     = 34
	)

	idLen := idPadding

	if !p.Enabled() {
		idLen = idWithColor
	}

	idStr := strconv.Itoa(b.ID)
	paddedID := fmt.Sprintf("%*s", idLen, idStr)
	coloredID := strings.Replace(paddedID, idStr, p.BrightYellow.Wrap(idStr, p.Bold), 1)
	urlColor := parametersHighlight(b.URL, p.BrightRed.With(p.Italic).Sprint, p.Dim.Sprint)

	var sb strings.Builder
	sb.Grow(w + 20)
	sb.WriteString(coloredID)
	sb.WriteByte(' ')
	sb.WriteString(c.Glyphs().Sep)
	sb.WriteByte(' ')
	sb.WriteString(urlColor)

	return sb.String()
}

func MiniFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	const (
		idWidth = 3
		gap     = " "
		minURL  = 40
	)

	idRaw := strconv.Itoa(b.ID)
	idStr := fmt.Sprintf("%*s", idWidth, idRaw)
	if p.Enabled() {
		idStr = p.Dim.Sprint(idStr)
	}

	flags := formatFlags(c.Glyphs(), b)
	flagsStr := ""
	if flags != "" {
		flags = strings.TrimSpace(flags)
		if p.Enabled() {
			flagsStr = p.BrightMagenta.Wrap(flags, p.Bold)
		} else {
			flagsStr = flags
		}
	}

	reserved := idWidth + 1 // ID + space
	if flagsStr != "" {
		reserved += len(flags) + 1
	} else {
		reserved += 2 // consistent visual spacing
	}

	urlMax := w - reserved - 2 // tags margin
	urlMax = max(urlMax, 20)

	shortURL := txt.Shorten(b.URL, urlMax)

	urlStr := shortURL
	if p.Enabled() {
		urlStr = p.BrightCyan.Sprint(shortURL)
	}

	urlWidth := txt.StringWidth(shortURL)
	if urlWidth < minURL {
		padding := minURL - urlWidth
		urlStr += strings.Repeat(" ", padding)
	}

	tagsStr := ""
	if b.Tags != "" {
		tags := txt.TagsWith(b.Tags, c.Glyphs().Sep) // "#tag #tag"
		if p.Enabled() {
			tagsStr = p.Dim.Sprint(tags)
		} else {
			tagsStr = tags
		}
	}

	var sb strings.Builder
	sb.Grow(w)

	sb.WriteString(idStr)
	sb.WriteString(gap)

	if flagsStr != "" {
		sb.WriteString(flagsStr)
		sb.WriteString(gap)
	} else {
		sb.WriteString("  ")
	}

	sb.WriteString(urlStr)

	if tagsStr != "" {
		sb.WriteString("  ")
		sb.WriteString(tagsStr)
	}

	return sb.String()
}

// MinimalFunc formats a bookmark with a focus on readability and clean spacing.
// Layout:  ID  [Flags]  Title  (domain)  #tags.
func MinimalFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	// 1. ID with subtle color
	idStr := fmt.Sprintf("%03d", b.ID)
	coloredID := p.Dim.Sprint(idStr)

	// 2. Flags (Single character indicator)
	// We use color to distinguish flags rather than a long string
	flag := " "
	switch {
	case b.Favorite:
		flag = p.BrightYellow.Sprint("★")
	case b.Notes != "":
		flag = p.BrightCyan.Sprint("•")
	case b.HTTPStatusCode >= 400:
		flag = p.Red.Sprint("!")
	}

	// 3. Content: Title and Domain
	displayTitle := strings.ReplaceAll(b.Title, "\n", " ")
	if displayTitle == "" {
		displayTitle = b.URL
	}

	// Extract domain for a cleaner look
	domain := ""
	if u, err := url.Parse(b.URL); err == nil {
		domain = p.Dim.Sprintf("(%s)", u.Host)
	}

	// 4. Tags
	tags := ""
	if b.Tags != "" {
		tags = p.Blue.Sprint("#" + strings.ReplaceAll(b.Tags, ",", " #"))
	}

	// Calculate spacing to keep tags aligned to the right or
	// just a few spaces after the domain.
	line := fmt.Sprintf(
		"%s %s %s %s %s",
		coloredID,
		flag,
		displayTitle,
		domain,
		tags,
	)

	return txt.Shorten(line, w)
}

// CardLiteFunc formats a bookmark in two thin lines.
// Line 1: [ID] Title (Flags)
// Line 2:      URL (dimmed) • #tags.
func CardLiteFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	// --- Line 1: The Heading ---
	idStr := p.BrightYellow.Sprintf("%d", b.ID)

	// Title handling
	title := b.Title
	if title == "" {
		title = "Untitled"
	}
	title = strings.ReplaceAll(title, "\n", " ")

	flags := formatColorFlags(p, c.Glyphs(), b)
	line1 := fmt.Sprintf("%s %s %s", idStr, title, flags)

	// --- Line 2: The Context ---
	// Shorten URL and dim it
	shortURL := txt.Shorten(b.URL, w/2)
	dimURL := p.Dim.Sprint(shortURL)

	// Tags with a subtle separator
	tags := ""
	if b.Tags != "" {
		tags = " " +
			c.Glyphs().Sep +
			" " +
			p.Blue.Sprint(txt.TagsWith(b.Tags, c.Glyphs().Sep))
	}

	// Indent line 2 to align under the title (past the ID)
	indent := strings.Repeat(" ", len(strconv.Itoa(b.ID))+1)
	line2 := fmt.Sprintf("%s%s%s", indent, dimURL, tags)

	return line1 + "\n" + line2
}

// FlowFunc formats a bookmark as a single continuous path.
// Layout: ID › Title - domain #tags.
func FlowFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	idStyle := p.Dim.Sprint
	domainStyle := p.Dim.Sprint
	tagStyle := p.Blue.Sprint

	idPart := idStyle(fmt.Sprintf("%03d", b.ID))

	titlePart := strings.ReplaceAll(b.Title, "\n", " ")
	if titlePart == "" {
		titlePart = "Untitled"
	}

	g := c.Glyphs()
	sep := " " + g.Pointer + " "
	if b.Favorite {
		sep = p.BrightYellow.Sprintf(" %s ", g.RightDoubleAngle)
	} else if b.HTTPStatusCode >= 400 {
		sep = p.Red.Sprint(" ! ")
	}

	domain := ""
	if u, err := url.Parse(b.URL); err == nil {
		domain = " — " + u.Host
	}

	tags := ""
	if b.Tags != "" {
		tags = tagStyle(txt.TagsWithPound(b.Tags))
	}

	line := fmt.Sprintf(
		"%s%s%s%s %s",
		idPart,
		sep,
		titlePart,
		domainStyle(domain),
		tags,
	)

	return txt.Shorten(line, w)
}

// BarFunc formats a bookmark as a clean dashboard-style entry.
// Layout: ┃ ID Title  [tags] ... domain.
func BarFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	gutterStyle := p.Dim
	switch {
	case b.Favorite:
		gutterStyle = p.BrightYellow
	case b.HTTPStatusCode >= 400:
		gutterStyle = p.Red
	case b.Notes != "":
		gutterStyle = p.BrightCyan
	}

	idStr := fmt.Sprintf("%03d", b.ID)

	titlePlain := strings.ReplaceAll(b.Title, "\n", " ")
	if titlePlain == "" {
		titlePlain = b.URL
	}

	tagsPlain := ""
	if b.Tags != "" {
		// FIX: it has a remaining space.
		b.Tags = strings.TrimRight(b.Tags, ",")
		tagsPlain = fmt.Sprintf("[%s]", strings.ReplaceAll(b.Tags, ",", " "))
	}

	domainPlain := ""
	if u, err := url.Parse(b.URL); err == nil {
		domainPlain = u.Host
	}

	// ┃ + space + ID(3) + space + space + [tags] + space + space + domain
	// calculate "occupied" width to see how much title we can fit
	staticWidth := 1 + 1 + 3 + 1 + 1 + txt.StringWidth(tagsPlain) + 2 + txt.StringWidth(domainPlain)

	// title truncation
	maxTitleW := w - staticWidth
	if maxTitleW < 10 { // if it's too cramped, hide tags to save space
		tagsPlain = ""
		staticWidth = 1 + 1 + 3 + 1 + 1 + 2 + txt.StringWidth(domainPlain)
		maxTitleW = w - staticWidth
	}

	titleTrunc := txt.Shorten(titlePlain, max(maxTitleW, 5))

	gutter := gutterStyle.Sprint(c.Glyphs().HeavyVertical)
	idCol := p.Dim.Sprint(idStr)

	var titleCol string
	if b.Title == "" {
		titleCol = p.Dim.Wrap(titleTrunc, p.Italic)
	} else {
		titleCol = p.Bold.Sprint(titleTrunc)
	}

	tagsCol := ""
	if tagsPlain != "" {
		tagsCol = p.Blue.Sprint(tagsPlain)
	}

	// calculate spacer (the dots)
	// current width = gutter(1) + id(3) + title + tags + domain + spaces
	currentVisualWidth := 1 + 1 + 3 + 1 + txt.StringWidth(
		titleTrunc,
	) + 1 + txt.StringWidth(
		tagsPlain,
	) + 1 + txt.StringWidth(
		domainPlain,
	)
	dotCount := w - currentVisualWidth

	dots := ""
	if dotCount > 0 {
		dots = p.Dim.Sprint(strings.Repeat(".", dotCount))
	}

	return fmt.Sprintf(
		"%s %s %s %s %s %s",
		gutter,
		idCol,
		titleCol,
		tagsCol,
		dots,
		p.Dim.Sprint(domainPlain),
	)
}

func ArchiveURLFunc(c Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	absolute, relative, err := txt.TimeWithAgo(b.ArchiveTimestamp)
	if err != nil {
		absolute = "---- --- -- -----"
		relative = "error"
	}

	year, rest, _ := strings.Cut(absolute, " ")
	year = yearColor(year, p).Wrap(year, p.Bold)

	actual, _ := extractArchiveURL(b.URL)
	domain, err := bookmark.Domain(actual)
	if err != nil {
		domain = "no-domain"
	}
	domain = p.Dim.Sprintf("(%s)", domain)

	idStr := p.Dim.Sprint(b.ID)
	title := p.Normal.Sprint(strings.ReplaceAll(b.Title, "\n", " "))
	if b.Title == "" {
		title = p.Dim.Sprint(b.URL)
	}

	yearWidth := txt.StringWidth(year)
	restWidth := txt.StringWidth(rest)
	domainWidth := txt.StringWidth(domain)
	idWidth := txt.StringWidth(idStr)

	reservedWidth := yearWidth + restWidth
	maxTitleWidth := reservedWidth + domainWidth/2

	title = txt.Shorten(title, w-maxTitleWidth-6)
	relative = p.BrightYellow.Wrap("("+relative+")", p.Italic)

	padding := reservedWidth + idWidth - 6

	return fmt.Sprintf(
		"%s %s %s %-*s %s %s",
		idStr,
		year,
		rest,
		padding,
		relative,
		title,
		domain,
	)
}

func NotesFunc(c Console, b *bookmark.Bookmark) string {
	const labelWidth = 8

	maxWidth := c.MinWidth()
	p := c.Palette()

	var header string
	if b.Title != "" {
		header = txt.SplitAndAlign(b.Title, maxWidth, 0)
	} else {
		header = b.URL
	}

	field := func(label, value string) string {
		return txt.PaddedLineWithWidth(
			p.Dim.Sprint(label+":"),
			value,
			labelWidth,
		)
	}

	f := frame.New(
		frame.WithWriter(c.Writer()),
		frame.WithBorders(frame.NewBorders("", "", "", "")),
	)

	g := c.Glyphs()
	tags := txt.TagsWithPound(b.Tags)
	tags = txt.TagsColoredWithDelimiters(
		p,
		strings.Split(tags, " "),
		g.SeparatorLeft,
		g.SeparatorRight,
	)

	f.Ln().
		Headerln(p.BgBlue.Wrap(header, p.Black, p.Bold)).
		Rowln(field("ID", p.Bold.Sprint(strconv.Itoa(b.ID)))).
		Rowln(field("Tags", tags)).
		Rowln(field("URL", p.BrightCyan.Wrap(b.URL, p.Bold, p.Underline)))

	if b.Desc != "" {
		desc := txt.SplitAndAlign(b.Desc, maxWidth, labelWidth+1)
		f.Rowln(field("Desc", desc))
	}

	return f.Text(b.Notes).StringReset()
}

type fieldSpec struct {
	name  string
	limit int // 0: no limit
}

func ByFields(c Console, bs []*bookmark.Bookmark, fieldsInput string) error {
	// parse input: "id,url:40,title:40"
	parts := strings.Split(fieldsInput, ",")
	specs := make([]fieldSpec, len(parts))

	for i, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, ":") {
			sub := strings.Split(p, ":")
			specs[i].name = sub[0]
			specs[i].limit, _ = strconv.Atoi(sub[1])
		} else {
			specs[i].name = p
		}
	}

	w := tabwriter.NewWriter(c.Writer(), 0, 0, 2, ' ', 0)

	for _, b := range bs {
		var row []string

		for _, spec := range specs {
			val, err := b.Field(spec.name)
			if err != nil {
				return err
			}

			if spec.limit > 0 {
				val = txt.Shorten(val, spec.limit)
			} else {
				safeLimit := c.MaxWidth() / len(specs)
				val = txt.Shorten(val, safeLimit)
			}

			row = append(row, val)
		}

		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	return w.Flush()
}

// StatusCodeFunc formats a bookmark with its HTTP status and URL.
func StatusCodeFunc(c Console, b *bookmark.Bookmark) string {
	const statusWidth = 22

	p := c.Palette()

	statusText := b.HTTPStatusText
	if statusText == "" {
		statusText = "Unassigned"
	}

	statusText = txt.Shorten(statusText, statusWidth-6)

	statusLabel := fmt.Sprintf(
		"(%d) %s",
		b.HTTPStatusCode,
		statusText,
	)

	bURL := txt.Shorten(
		b.URL,
		c.MaxWidth()-statusWidth,
	)

	var sb strings.Builder

	sb.WriteString(p.Bold.Sprintf("%-*d ", 4, b.ID))

	sb.WriteString(
		txt.PaddedLineWithWidth(
			txt.HTTPStatusCodeColor(
				b.HTTPStatusCode,
				p,
			).Sprint(statusLabel),
			bURL,
			statusWidth,
		),
	)

	return sb.String()
}

// formatFlags returns a string representation of bookmark status flags.
func formatFlags(g *Glyphs, b *bookmark.Bookmark) string {
	var flags strings.Builder
	if b.Favorite {
		flags.WriteString(g.Favorite)
	}
	if b.Notes != "" {
		flags.WriteString(g.Notes)
	}
	if b.ArchiveURL != "" {
		flags.WriteString(g.Archive)
	}
	if b.HTTPStatusCode == http.StatusNotFound {
		flags.WriteString(g.Broken)
	}
	if !b.IsActive {
		flags.WriteString(g.Inactive)
	}

	return flags.String()
}

// formatColorFlags returns a string representation of bookmark status flags.
func formatColorFlags(p *ansi.Palette, g *Glyphs, b *bookmark.Bookmark) string {
	var flags strings.Builder
	if b.Favorite {
		flags.WriteString(p.BrightYellow.Code() + g.Favorite)
	}
	if b.Notes != "" {
		flags.WriteString(p.BrightCyan.Code() + g.Notes)
	}
	if b.ArchiveURL != "" {
		flags.WriteString(p.BrightMagenta.Code() + g.Archive)
	}
	if b.HTTPStatusCode == http.StatusNotFound {
		flags.WriteString(p.BrightRed.Code() + g.Broken)
	}
	if !b.IsActive {
		flags.WriteString(p.Orange.Code() + g.Inactive)
	}

	return p.Dim.Sprint(flags.String())
}

func extractArchiveURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if u.Host == "web.archive.org" {
		// Path is: /web/YYYYMMDDXXXXXX/https://github.com/mateconpizza/gm
		parts := strings.SplitN(u.Path, "/", 4)
		if len(parts) >= 4 {
			return parts[3], nil
		}
	}

	return urlStr, nil
}

func yearColor(year string, p *ansi.Palette) ansi.Style {
	const (
		startYear = 2000
		endYear   = 2050
	)

	y, err := strconv.Atoi(year)
	if err != nil || y < startYear || y > endYear {
		return p.BrightYellow
	}

	colorCycle := []ansi.Style{
		p.Cyan,
		p.BrightCyan,
		p.Blue,
		p.BrightBlue,
		p.Magenta,
		p.BrightMagenta,
		p.Red,
		p.BrightRed,
		p.Green,
		p.BrightGreen,
	}

	// calculate the index offset from your starting year (2000)
	index := (y - startYear) % len(colorCycle)

	return colorCycle[index]
}

// parametersHighlight returns the URL with its query parameters highlighted with
// the given color func.
func parametersHighlight(raw string, hlFn, mutedFn func(a ...any) string) string {
	u, err := url.Parse(raw)
	if err != nil || u.RawQuery == "" {
		return raw
	}

	// preserve original ordering
	parts := strings.Split(u.RawQuery, "&")

	var highlighted []string
	highlighted = make([]string, 0, len(parts))

	for _, p := range parts {
		if p == "" {
			continue
		}

		kv := strings.SplitN(p, "=", 2)

		if len(kv) == 1 {
			// parameter without value: ?flag
			highlighted = append(highlighted, hlFn(kv[0]))
			continue
		}

		key := kv[0]
		val := kv[1]

		colored := hlFn(key + "=" + val)
		highlighted = append(highlighted, colored)
	}

	// rebuild manually so we don't lose encoding or formatting
	var sb strings.Builder
	sb.Grow(len(raw) + len(parts)*10)

	// base URL without query
	base := raw[:strings.Index(raw, "?")+1]
	sb.WriteString(mutedFn(base))
	sb.WriteString(strings.Join(highlighted, "&"))

	return sb.String()
}
