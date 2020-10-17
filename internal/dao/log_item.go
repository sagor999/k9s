package dao

import (
	"bytes"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/derailed/k9s/internal/color"
	"github.com/rs/zerolog/log"
	"github.com/sahilm/fuzzy"
)

// LogChan represents a channel for logs.
type LogChan chan *LogItem

// LogItem represents a container log line.
type LogItem struct {
	Pod, Container, Timestamp string
	SingleContainer           bool
	Bytes                     []byte
}

// NewLogItem returns a new item.
func NewLogItem(b []byte) *LogItem {
	space := []byte(" ")
	var l LogItem

	cols := bytes.Split(b[:len(b)-1], space)
	l.Timestamp = string(cols[0])
	l.Bytes = bytes.Join(cols[1:], space)

	return &l
}

// NewLogItemFromString returns a new item.
func NewLogItemFromString(s string) *LogItem {
	return &LogItem{
		Bytes:     []byte(s),
		Timestamp: time.Now().String(),
	}
}

// ID returns pod and or container based id.
func (l *LogItem) ID() string {
	if l.Pod != "" {
		return l.Pod
	}
	return l.Container
}

// Clone copies an item.
func (l *LogItem) Clone() *LogItem {
	bytes := make([]byte, len(l.Bytes))
	copy(bytes, l.Bytes)
	return &LogItem{
		Container:       l.Container,
		Pod:             l.Pod,
		Timestamp:       l.Timestamp,
		SingleContainer: l.SingleContainer,
		Bytes:           bytes,
	}
}

// Info returns pod and container information.
func (l *LogItem) Info() string {
	return fmt.Sprintf("%q::%q", l.Pod, l.Container)
}

// IsEmpty checks if the entry is empty.
func (l *LogItem) IsEmpty() bool {
	return len(l.Bytes) == 0
}

var (
	escPattern = regexp.MustCompile(`(\[[a-zA-Z0-9_,;: \-\."#]+\[*)\]`)
	matcher    = []byte("$1[]")
)

// Render returns a log line as string.
func (l *LogItem) Render(c int, showTime bool) []byte {
	bb := make([]byte, 0, 200)
	if showTime {
		t := l.Timestamp
		for i := len(t); i < 30; i++ {
			t += " "
		}
		bb = append(bb, color.ANSIColorize(t, 106)...)
		bb = append(bb, ' ')
	}

	if l.Pod != "" {
		bb = append(bb, color.ANSIColorize(l.Pod, c)...)
		bb = append(bb, ':')
	}
	if !l.SingleContainer && l.Container != "" {
		bb = append(bb, color.ANSIColorize(l.Container, c)...)
		bb = append(bb, ' ')
	}

	return append(bb, escPattern.ReplaceAll(l.Bytes, matcher)...)
}

func colorFor(n string) int {
	var sum int
	for _, r := range n {
		sum += int(r)
	}

	c := sum % 256
	if c == 0 {
		c = 207 + rand.Intn(10)
	}
	return c
}

// ----------------------------------------------------------------------------

// LogItems represents a collection of log items.
type LogItems []*LogItem

// Lines returns a collection of log lines.
func (l LogItems) Lines() []string {
	ll := make([]string, len(l))
	for i, item := range l {
		ll[i] = string(item.Render(0, false))
	}

	return ll
}

// Render returns logs as a collection of strings.
func (l LogItems) Render(showTime bool, ll [][]byte) {
	colors := make(map[string]int, len(l))
	for i, item := range l {
		info := item.ID()
		color, ok := colors[info]
		if !ok {
			color = colorFor(info)
			colors[info] = color
		}
		ll[i] = item.Render(color, showTime)
	}
}

// DumpDebug for debuging
func (l LogItems) DumpDebug(m string) {
	fmt.Println(m + strings.Repeat("-", 50))
	for i, line := range l {
		fmt.Println(i, string(line.Bytes))
	}
}

// Filter filters out log items based on given filter.
func (l LogItems) Filter(q string) ([]int, [][]int, error) {
	if q == "" {
		return nil, nil, nil
	}
	if IsFuzzySelector(q) {
		mm, ii := l.fuzzyFilter(strings.TrimSpace(q[2:]))
		return mm, ii, nil
	}
	matches, indices, err := l.filterLogs(q)
	if err != nil {
		log.Error().Err(err).Msgf("Logs filter failed")
		return nil, nil, err
	}
	return matches, indices, nil
}

var fuzzyRx = regexp.MustCompile(`\A\-f`)

func (l LogItems) fuzzyFilter(q string) ([]int, [][]int) {
	q = strings.TrimSpace(q)
	matches, indices := make([]int, 0, len(l)), make([][]int, 0, 10)
	mm := fuzzy.Find(q, l.Lines())
	for _, m := range mm {
		matches = append(matches, m.Index)
		indices = append(indices, m.MatchedIndexes)
	}

	return matches, indices
}

func (l LogItems) filterLogs(q string) ([]int, [][]int, error) {
	rx, err := regexp.Compile(`(?i)` + q)
	if err != nil {
		return nil, nil, err
	}
	matches, indices := make([]int, 0, len(l)), make([][]int, 0, 10)
	for i, line := range l.Lines() {
		if locs := rx.FindStringIndex(line); locs != nil {
			matches = append(matches, i)
			ii := make([]int, 0, 10)
			for i := 0; i < len(locs); i += 2 {
				for j := locs[i]; j < locs[i+1]; j++ {
					ii = append(ii, j)
				}
			}
			indices = append(indices, ii)
		}
	}

	return matches, indices, nil
}
