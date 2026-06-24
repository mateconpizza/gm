package formatter

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"

	runewidth "github.com/mattn/go-runewidth"

	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

// OnelineFunc formats a bookmark in a single line with the given colorscheme.
//
//	ID • URL  #go #tools.
func OnelineFunc(c *ui.Console, b *bookmark.Bookmark) string {
	w := c.MaxWidth()

	const (
		idPadding      = 3
		idWithColor    = 4 // visible width for IDS up to 9999
		defaultTagsLen = 24
		minTagsLen     = 34
	)

	idLen := idPadding
	tagsLen := minTagsLen

	p := c.Palette()
	if !p.Enabled() {
		idLen = idWithColor
		tagsLen = defaultTagsLen
	}

	// ID padding con color sin romper el formato
	idStr := strconv.Itoa(b.ID)
	paddedID := fmt.Sprintf("%*s", idLen, idStr)
	coloredID := strings.Replace(paddedID, idStr, p.BrightYellow.Wrap(idStr, p.Bold), 1)

	// Calculate long available for URL
	const urlPadding = 3 // 3 = ' ' + '·' + ' '.
	urlLen := w - idLen - urlPadding - tagsLen
	shortURL := txt.Shorten(b.URL, urlLen)
	colorURL := p.Dim.Sprint(shortURL)
	urlLen += len(colorURL) - len(shortURL)

	// tags
	tagsColor := p.Blue.Wrap(txt.TagsWith(b.Tags, txt.GlyphMiddleDot.String()), p.Italic)

	sep := txt.GlyphMiddleDot.With(func(g txt.Glyph) string {
		return " " + txt.GlyphMiddleDot.String() + " "
	})
	if b.Notes != "" || b.Favorite || b.ArchiveURL != "" {
		sep = p.BrightMagenta.Wrap(txt.GlyphMiddleDot.With(func(u txt.Glyph) string {
			return " " + u.String() + " "
		}), p.Bold)
	}

	var sb strings.Builder
	sb.Grow(w + 20)
	sb.WriteString(coloredID)
	sb.WriteString(sep)
	fmt.Fprintf(&sb, "%-*s %-*s\n", urlLen, colorURL, tagsLen, tagsColor)

	return sb.String()
}

// BriefFunc formats a bookmark as a simple, clean list item.
//
//	┃ ID Title (domain) #go #tools.
func BriefFunc(c *ui.Console, b *bookmark.Bookmark) string {
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
	domainWidth := runewidth.StringWidth(domainPlain)

	tagsPlain := ""
	if b.Tags != "" {
		tagsPlain = txt.TagsWith(b.Tags, txt.GlyphMiddleDot.String())
	}

	// width = total - (bullet + id + domain + tags + 3 spaces)
	overhead := bulletWidth + idMaxWidth + domainWidth + tagsBudget + spacing
	maxTitleWidth := max(w-overhead, 1)

	rawTitle := b.Title
	if rawTitle == "" {
		rawTitle = b.URL
	}
	truncatedTitle := runewidth.Truncate(rawTitle, maxTitleWidth, "…")
	// ensure the title block always occupies exactly maxtitlewidth on screen
	paddedTitle := runewidth.FillRight(truncatedTitle, maxTitleWidth)

	// bullet
	g := txt.GlyphHeavyVertical
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
		"%s %s %s%s  %s\n",
		bulletColored,
		p.Dim.Sprintf("%-*s", idMaxWidth, idStr),
		titleColored,
		domainColored,
		tagsColored,
	)
}

// MultilineFunc formats a bookmark for fzf with max width.
func MultilineFunc(c *ui.Console, b *bookmark.Bookmark) string {
	p, w := c.Palette(), c.MaxWidth()

	var sb strings.Builder
	sb.WriteString(p.BrightYellow.With(p.Bold).Sprint(b.ID))
	sb.WriteString(txt.NBSP)
	sb.WriteString(txt.URLBreadCrumbsColor(p, b.URL, txt.GlyphSingleAngleMark.String(), w))
	sb.WriteString("\n")

	if b.Title != "" {
		sb.WriteString(p.Cyan.Sprint(txt.Shorten(b.Title, w)))
		sb.WriteString("\n")
	}

	sb.WriteString(p.BrightWhite.Wrap(txt.TagsWith(b.Tags, txt.GlyphMiddleDot.String()), p.Italic))

	return sb.String()
}

// FrameFunc formats a bookmark with a frame.
//
//   - ID [FLAGS] URL
//   - Title
//   - Desc
//   - Tags
func FrameFunc(c *ui.Console, b *bookmark.Bookmark) string {
	w, p, f := c.MaxWidth(), c.Palette(), c.Frame()

	// initial border adjustment
	borders := f.Borders()
	w -= len(borders.Row)

	idStr := strconv.Itoa(b.ID)
	// calculate visual width of id
	usedWidth := runewidth.StringWidth(idStr)

	idColor := p.BrightYellow.With(p.Bold).Sprint(idStr)
	header := []string{idColor}

	// prepare flags (if any) and accumulate width
	if flags := formatFlags(b); flags != "" {
		// " [" + flags + "]"
		flagRaw := "[" + flags + "]"
		header = append(header, p.Dim.Sprint(flagRaw))

		// add flag width + 1 (for the space strings.join will add)
		usedWidth += runewidth.StringWidth(flagRaw) + 1
	}

	// calculate space for url
	// we subtract 'usedwidth' and 1 extra for the final space before the url
	urlWidth := w - usedWidth - 1

	header = append(header, txt.URLBreadCrumbsColor(p, b.URL, txt.GlyphSingleAngleMark.String(), urlWidth))
	f.Midln(strings.Join(header, " "))

	if b.Title != "" {
		titleSplit := txt.SplitIntoChunks(b.Title, w)
		f.Midln(ansi.StyleAll(titleSplit, p.BrightCyan)...)
	}

	if b.Desc != "" {
		descSplit := txt.SplitIntoChunks(b.Desc, w)
		f.Midln(ansi.StyleAll(descSplit, p.Dim)...)
	}

	f.Footer(txt.TagsWithColorPound(c, b.Tags)).Ln()

	return f.StringReset()
}

func OnelineURLFunc(c *ui.Console, b *bookmark.Bookmark) string {
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

	var sb strings.Builder
	sb.Grow(w + 20)
	sb.WriteString(coloredID)
	sb.WriteString(txt.GlyphMiddleDot.With(func(u txt.Glyph) string {
		return " " + u.String() + ""
	}))
	sb.WriteString(b.URL)
	sb.WriteString("\n")

	return sb.String()
}

func MiniFunc(c *ui.Console, b *bookmark.Bookmark) string {
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

	flags := formatFlags(b)
	flagsStr := ""
	if flags != "" {
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

	urlWidth := runewidth.StringWidth(shortURL)
	if urlWidth < minURL {
		padding := minURL - urlWidth
		urlStr += strings.Repeat(" ", padding)
	}

	tagsStr := ""
	if b.Tags != "" {
		tags := txt.TagsWith(b.Tags, txt.GlyphMiddleDot.String()) // "#tag #tag"
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

	sb.WriteByte('\n')

	return sb.String()
}

// MinimalFunc formats a bookmark with a focus on readability and clean spacing.
// Layout:  ID  [Flags]  Title  (domain)  #tags.
func MinimalFunc(c *ui.Console, b *bookmark.Bookmark) string {
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
	displayTitle := b.Title
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

	// Trim if it exceeds terminal width
	return runewidth.Truncate(line, w, "…") + "\n"
}

// CardLiteFunc formats a bookmark in two thin lines.
// Line 1: [ID] Title (Flags)
// Line 2:      URL (dimmed) • #tags.
func CardLiteFunc(c *ui.Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	// --- Line 1: The Heading ---
	idStr := p.BrightYellow.Sprintf("%d", b.ID)

	// Title handling
	title := b.Title
	if title == "" {
		title = "Untitled"
	}
	title = p.Bold.Sprint(title)

	// Minimalist Flag icons
	flags := ""
	if b.Favorite {
		flags += " " + p.BrightYellow.Sprint("★")
	}
	if b.Notes != "" {
		flags += " " + p.BrightCyan.Sprint("✎")
	}
	if b.ArchiveURL != "" {
		flags += " " + p.Dim.Sprint("∞")
	}

	line1 := fmt.Sprintf("%s %s%s", idStr, title, flags)

	// --- Line 2: The Context ---
	// Shorten URL and dim it
	shortURL := txt.Shorten(b.URL, w/2)
	dimURL := p.Dim.Sprint(shortURL)

	// Tags with a subtle separator
	tags := ""
	if b.Tags != "" {
		tags = " " +
			txt.GlyphMiddleDot.String() +
			" " +
			p.Blue.Sprint(txt.TagsWith(b.Tags, txt.GlyphMiddleDot.String()))
	}

	// Indent line 2 to align under the title (past the ID)
	indent := strings.Repeat(" ", len(strconv.Itoa(b.ID))+1)
	line2 := fmt.Sprintf("%s%s%s", indent, dimURL, tags)

	return line1 + "\n" + line2 + "\n\n"
}

// FlowFunc formats a bookmark as a single continuous path.
// Layout: ID › Title — domain #tags.
func FlowFunc(c *ui.Console, b *bookmark.Bookmark) string {
	w, p := c.MaxWidth(), c.Palette()

	idStyle := p.Dim.Sprint
	domainStyle := p.Dim.Sprint
	tagStyle := p.Blue.Sprint

	idPart := idStyle(fmt.Sprintf("%03d", b.ID))

	titlePart := b.Title
	if titlePart == "" {
		titlePart = "Untitled"
	}

	sep := " › "
	if b.Favorite {
		sep = p.BrightYellow.Sprint(" » ")
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

	return runewidth.Truncate(line, w, "…") + "\n"
}

// BarFunc formats a bookmark as a clean dashboard-style entry.
// Layout: ┃ ID Title  [tags] ... domain.
func BarFunc(c *ui.Console, b *bookmark.Bookmark) string {
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

	titlePlain := b.Title
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
	staticWidth := 1 + 1 + 3 + 1 + 1 + runewidth.StringWidth(tagsPlain) + 2 + runewidth.StringWidth(domainPlain)

	// title truncation
	maxTitleW := w - staticWidth
	if maxTitleW < 10 { // if it's too cramped, hide tags to save space
		tagsPlain = ""
		staticWidth = 1 + 1 + 3 + 1 + 1 + 2 + runewidth.StringWidth(domainPlain)
		maxTitleW = w - staticWidth
	}

	titleTrunc := runewidth.Truncate(titlePlain, max(maxTitleW, 5), "…")

	gutter := gutterStyle.Sprint(txt.GlyphHeavyVertical)
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
	currentVisualWidth := 1 + 1 + 3 + 1 + runewidth.StringWidth(
		titleTrunc,
	) + 1 + runewidth.StringWidth(
		tagsPlain,
	) + 1 + runewidth.StringWidth(
		domainPlain,
	)
	dotCount := w - currentVisualWidth

	dots := ""
	if dotCount > 0 {
		dots = p.Dim.Sprint(strings.Repeat(".", dotCount))
	}

	return fmt.Sprintf(
		"%s %s %s %s %s %s\n",
		gutter,
		idCol,
		titleCol,
		tagsCol,
		dots,
		p.Dim.Sprint(domainPlain),
	)
}

type fieldSpec struct {
	name  string
	limit int // 0: no limit
}

func ByFields(c *ui.Console, bs []*bookmark.Bookmark, fieldsInput string) error {
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

// formatFlags returns a string representation of bookmark status flags.
//
//	~ Notes attached
//	@ Wayback snapshot available
//	* Favorite
//	? Broken link
func formatFlags(b *bookmark.Bookmark) string {
	const (
		wayback   = "@" // @
		tilde     = "~" // ~
		broken    = "?" // ?
		important = "*" // *
	)
	var flags strings.Builder

	if b.ArchiveURL != "" {
		flags.WriteString(wayback)
	}
	if b.Notes != "" {
		flags.WriteString(tilde)
	}
	if b.Favorite {
		flags.WriteString(important)
	}
	if b.HTTPStatusCode == http.StatusNotFound {
		flags.WriteString(broken)
	}

	return flags.String()
}

// StatusCodeFunc formats a bookmark with its HTTP status and URL.
func StatusCodeFunc(c *ui.Console, b *bookmark.Bookmark) string {
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
		txt.PaddedLineWithPad(
			txt.HTTPStatusCodeColor(
				b.HTTPStatusCode,
				p,
			).Sprint(statusLabel),
			bURL,
			statusWidth,
		),
	)

	sb.WriteByte('\n')

	return sb.String()
}
