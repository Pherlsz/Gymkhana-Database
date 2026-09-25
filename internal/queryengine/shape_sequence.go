package queryengine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type chainItem struct {
	row      ShapeRow
	key      string
	along    int
	alongOK  bool
	distinct string
}

func applySequence(rows []ShapeRow, spec SequenceSpec) ([]ShapeRow, string) {
	if spec.Partition != "" {
		groups := map[string][]ShapeRow{}
		order := []string{}
		for _, row := range rows {
			key := row.Fields[spec.Partition]
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], row)
		}
		best := [][]ShapeRow{}
		bestLength := 0
		for _, key := range order {
			chains, _ := oneSequence(groups[key], spec)
			length := 0
			for _, chain := range chains {
				if len(chain) > length {
					length = len(chain)
				}
			}
			if length > bestLength {
				best = chains
				bestLength = length
				continue
			}
			if length > 0 && length == bestLength {
				best = append(best, chains...)
			}
		}
		return finishChains(best, spec)
	}
	chains, _ := oneSequence(rows, spec)
	return finishChains(chains, spec)
}

func oneSequence(rows []ShapeRow, spec SequenceSpec) ([][]ShapeRow, string) {
	items := make([]chainItem, 0, len(rows))
	for _, row := range rows {
		item := chainItem{row: row, key: row.Fields[spec.By], distinct: row.Fields[spec.Distinct]}
		if spec.Along != nil {
			number, err := strconv.Atoi(strings.TrimSpace(row.Fields[spec.Along.Field]))
			if err == nil {
				item.along = number
				item.alongOK = true
			}
		}
		if item.key == "" {
			continue
		}
		items = append(items, item)
	}
	step := spec.Step
	if step == "" {
		step = "next"
	}
	alphabet := parseAlphabet(spec.Alphabet)
	switch step {
	case "increase":
		return [][]ShapeRow{chainRows(numericChain(items, 1))}, "crescente"
	case "decrease":
		return [][]ShapeRow{chainRows(numericChain(items, -1))}, "decrescente"
	case "either_monotonic":
		up := numericChain(items, 1)
		down := numericChain(items, -1)
		if len(down) > len(up) {
			return [][]ShapeRow{chainRows(down)}, "decrescente"
		}
		return [][]ShapeRow{chainRows(up)}, "crescente"
	default:
		direction := 0
		if spec.Along != nil && (spec.Along.Step == "either_monotonic" || spec.Along.Step == "") {
			return longestDirection(alphabetChain(items, alphabet, 1), alphabetChain(items, alphabet, -1)), ""
		}
		if spec.Along != nil && spec.Along.Step == "decrease" {
			direction = -1
		}
		if spec.Along != nil && spec.Along.Step == "increase" {
			direction = 1
		}
		return chainGroups(alphabetChain(items, alphabet, direction)), ""
	}
}

func longestDirection(up, down [][]chainItem) [][]ShapeRow {
	upLen, downLen := groupLength(up), groupLength(down)
	if downLen > upLen {
		return chainGroups(down)
	}
	return chainGroups(up)
}

func groupLength(groups [][]chainItem) int {
	longest := 0
	for _, group := range groups {
		if len(group) > longest {
			longest = len(group)
		}
	}
	return longest
}

func chainGroups(groups [][]chainItem) [][]ShapeRow {
	rows := make([][]ShapeRow, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, chainRows(group))
	}
	return rows
}

func finishChains(chains [][]ShapeRow, spec SequenceSpec) ([]ShapeRow, string) {
	chains = breakTies(chains, spec)
	rows := []ShapeRow{}
	for _, chain := range chains {
		rows = append(rows, chain...)
	}
	summary := sequenceSummary(chains, spec)
	return rows, summary
}

func breakTies(chains [][]ShapeRow, spec SequenceSpec) [][]ShapeRow {
	if len(chains) <= 1 || spec.Tie == "" {
		return chains
	}
	best := chains[0]
	score := tieScore(best, spec)
	winners := [][]ShapeRow{best}
	for _, chain := range chains[1:] {
		value := tieScore(chain, spec)
		if value > score {
			score = value
			winners = [][]ShapeRow{chain}
			continue
		}
		if value == score {
			winners = append(winners, chain)
		}
	}
	return winners
}

func tieScore(chain []ShapeRow, spec SequenceSpec) int {
	switch spec.Tie {
	case "start":
		if len(chain) == 0 {
			return 0
		}
		key := chain[0].Fields[spec.By]
		if key == "" {
			return 0
		}
		return -int([]rune(key)[0])
	case "sum":
		total := 0
		for _, row := range chain {
			for _, value := range row.Fields {
				number, err := strconv.Atoi(strings.TrimSpace(value))
				if err == nil {
					total += number
				}
			}
		}
		return total
	default:
		seen := map[string]struct{}{}
		field := spec.Distinct
		for _, row := range chain {
			value := row.Fields[field]
			if field == "" {
				value = row.Label
			}
			if value != "" {
				seen[value] = struct{}{}
			}
		}
		return len(seen)
	}
}

func sequenceSummary(chains [][]ShapeRow, spec SequenceSpec) string {
	longest := 0
	for _, chain := range chains {
		if len(chain) > longest {
			longest = len(chain)
		}
	}
	if longest == 0 {
		text := "Nenhum registro formou sequência."
		if spec.Minimum > 0 {
			text += " O mínimo pedido é de " + strconv.Itoa(spec.Minimum) + "."
		}
		return text
	}
	text := "Maior sequência contínua: " + strconv.Itoa(longest) + " registros."
	if len(chains) > 1 {
		text += " Empate entre " + strconv.Itoa(len(chains)) + " sequências do mesmo tamanho."
	}
	if spec.Minimum > 0 {
		if longest >= spec.Minimum {
			text += " Atende o mínimo de " + strconv.Itoa(spec.Minimum) + "."
		} else {
			text += " Não atende o mínimo de " + strconv.Itoa(spec.Minimum) + "."
		}
	}
	return text
}

func chainRows(items []chainItem) []ShapeRow {
	rows := make([]ShapeRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, item.row)
	}
	return rows
}

func parseAlphabet(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		letters := make([]string, 0, 26)
		for letter := 'A'; letter <= 'Z'; letter++ {
			letters = append(letters, string(letter))
		}
		return letters
	}
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	sides := strings.Split(value, "-")
	if len(sides) == 2 && len(sides[0]) == 1 && len(sides[1]) == 1 {
		start, end := rune(sides[0][0]), rune(sides[1][0])
		if start > end {
			start, end = end, start
		}
		out := []string{}
		for letter := start; letter <= end; letter++ {
			out = append(out, string(letter))
		}
		return out
	}
	if len(sides) == 2 {
		start, startErr := strconv.Atoi(sides[0])
		end, endErr := strconv.Atoi(sides[1])
		if startErr == nil && endErr == nil && end >= start && end-start <= 64 {
			width := len(sides[0])
			out := make([]string, 0, end-start+1)
			for value := start; value <= end; value++ {
				out = append(out, fmt.Sprintf("%0*d", width, value))
			}
			return out
		}
	}
	return []string{value}
}

func alphabetChain(items []chainItem, alphabet []string, direction int) [][]chainItem {
	needsAlong := direction != 0
	grouped := map[string][]chainItem{}
	for _, item := range items {
		if needsAlong && !item.alongOK {
			continue
		}
		grouped[item.key] = append(grouped[item.key], item)
	}
	type node struct {
		length int
		prev   int
	}
	nodes := map[string][]node{}
	for index, key := range alphabet {
		group := grouped[key]
		sort.Slice(group, func(i, j int) bool {
			if group[i].along != group[j].along {
				if direction > 0 {
					return group[i].along < group[j].along
				}
				return group[i].along > group[j].along
			}
			return group[i].row.EntityID < group[j].row.EntityID
		})
		grouped[key] = group
		current := make([]node, len(group))
		var previous []chainItem
		var previousNodes []node
		if index > 0 {
			previous = grouped[alphabet[index-1]]
			previousNodes = nodes[alphabet[index-1]]
		}
		cursor := 0
		bestLength := 0
		bestIndex := -1
		for itemIndex, item := range group {
			for cursor < len(previous) && (direction == 0 || extendsNumber(direction, previous[cursor].along, item.along)) {
				if previousNodes[cursor].length > bestLength {
					bestLength = previousNodes[cursor].length
					bestIndex = cursor
				}
				cursor++
			}
			current[itemIndex] = node{length: 1, prev: -1}
			if bestIndex >= 0 {
				current[itemIndex] = node{length: bestLength + 1, prev: bestIndex}
			}
		}
		nodes[key] = current
	}
	bestLength := 0
	for _, key := range alphabet {
		for _, node := range nodes[key] {
			if node.length > bestLength {
				bestLength = node.length
			}
		}
	}
	if bestLength == 0 {
		return nil
	}
	chains := [][]chainItem{}
	for keyIndex, key := range alphabet {
		for index, node := range nodes[key] {
			if node.length != bestLength {
				continue
			}
			chain := make([]chainItem, 0, bestLength)
			cursorKey, cursor := keyIndex, index
			for cursor >= 0 && cursorKey >= 0 {
				chain = append(chain, grouped[alphabet[cursorKey]][cursor])
				cursor = nodes[alphabet[cursorKey]][cursor].prev
				cursorKey--
			}
			for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
				chain[left], chain[right] = chain[right], chain[left]
			}
			chains = append(chains, chain)
		}
	}
	return chains
}

func numericChain(items []chainItem, direction int) []chainItem {
	sorted := append([]chainItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		left, leftErr := strconv.Atoi(sorted[i].key)
		right, rightErr := strconv.Atoi(sorted[j].key)
		if leftErr != nil || rightErr != nil {
			if direction > 0 {
				return sorted[i].key < sorted[j].key
			}
			return sorted[i].key > sorted[j].key
		}
		if direction > 0 {
			return left < right
		}
		return left > right
	})
	best := []chainItem{}
	current := []chainItem{}
	for _, item := range sorted {
		number, err := strconv.Atoi(item.key)
		if err != nil {
			continue
		}
		if len(current) == 0 || extendsNumber(direction, mustAtoi(current[len(current)-1].key), number) {
			current = append(current, item)
			continue
		}
		if number == mustAtoi(current[len(current)-1].key) {
			continue
		}
		if len(current) > len(best) {
			best = current
		}
		current = []chainItem{item}
	}
	if len(current) > len(best) {
		best = current
	}
	return best
}

func extendsNumber(direction, previous, current int) bool {
	if direction > 0 {
		return previous < current
	}
	return previous > current
}

func mustAtoi(value string) int {
	number, _ := strconv.Atoi(value)
	return number
}
