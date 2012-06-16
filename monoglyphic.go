package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
)

var _ = fmt.Println

var wordListPath = flag.String("wordlist", "/usr/share/dict/words", "path to the word list")

type letterSet int32

func (this *letterSet) add(value uint8) {
	*this |= 1 << toIndex(value)
}

func (this *letterSet) contains(value uint8) bool {
	return (*this & (1 << toIndex(value))) == 1
}

func (this *letterSet) conflictsWith(other letterSet) bool {
	return (*this & other) != 0
}

func toLetterSet(input string) letterSet {
	result := letterSet(0)
	for _, character := range input {
		result.add(uint8(character))
	}
	return result
}

type trieNode struct {
	value    uint8
	parent   *trieNode
	terminal bool
	next     map[uint8]*trieNode
	used     letterSet
	partial  string
}

func newTrieNode(parent *trieNode) *trieNode {
	node := &trieNode{
		used:     letterSet(0),
		parent:   parent,
		terminal: false,
		next:     make(map[uint8]*trieNode),
		partial:  "",
	}

	if parent != nil {
		node.used = parent.used
	}

	return node
}

func (this *trieNode) insert(value string) {
	if len(value) == 0 {
		this.terminal = true
		return
	}

	character := value[0]
	if _, ok := this.next[character]; !ok {
		this.next[character] = newTrieNode(this)

		next := this.next[character]
		next.value = character
		next.used.add(character)
		next.partial = this.partial + value[0:1]
	}
	this.next[character].insert(value[1:])
}

func (this *trieNode) walk(value string) *trieNode {
	if len(value) == 0 {
		return this
	}

	character := value[0]
	next := this.next[character]
	if next != nil {
		return next.walk(value[1:])
	}
	return nil
}

func (this *trieNode) dump() {
	if this.terminal {
		fmt.Println(this.partial)
	}

	for _, next := range this.next {
		next.dump()
	}
}

func (this *trieNode) findUnconflictedTerminals(used letterSet, handler func(*trieNode)) {
	if this.used.conflictsWith(used) {
		return
	}

	if this.terminal {
		handler(this)
	}

	for _, next := range this.next {
		next.findUnconflictedTerminals(used, handler)
	}
}

func toIndex(input uint8) uint {
	return uint(input - 'a')
}

func validWord(input string) bool {
	// TODO(gdk): Find a better wordlist.
	if len(input) == 1 && input != "a" && input != "i" {
		return false
	}

	used := make([]bool, 26)

	for _, character := range input {
		if character < 'a' || character > 'z' {
			return false
		}

		index := toIndex(uint8(character))
		if used[index] {
			return false
		}
		used[index] = true
	}
	return true
}

func countWords(input string, root *trieNode) int {
	count := 0
	for i := range input {
		node := root
		for j := i; j < len(input); j++ {
			if node = node.walk(input[j : j+1]); node == nil {
				break
			}

			if node.terminal {
				count += 1
			}
		}
	}
	return count
}

var recordLength = 0
var recordPartial = ""

func augmentPartial(partial string, root *trieNode, wordList *[]string, depth int) {
	for i := len(partial); i >= 0; i-- {
		prefix, suffix := partial[:i], partial[i:]
		prefixSet := toLetterSet(prefix)
		suffixNode := root.walk(suffix)

		if suffixNode == nil {
			return
		}

		suffixHandler := func(node *trieNode) {
			validSuffix := node.partial
			if len(validSuffix) <= len(suffix) {
				return
			}

			length := countWords(prefix+validSuffix, root)
			if length > recordLength {
				recordLength = length
				recordPartial = prefix + validSuffix
				fmt.Println(recordPartial, recordLength)
			}

			newPartial := prefix + validSuffix
			augmentPartial(newPartial, root, wordList, depth+1)

		}
		suffixNode.findUnconflictedTerminals(prefixSet, suffixHandler)
	}
}

func handleRootSearch(root *trieNode, wordList *[]string, words chan string, done chan interface{}) {
	for word := range words {
		augmentPartial(word, root, wordList, 0)
		fmt.Println("done", word)
	}
	<-done
}

type wordScore struct {
	word  string
	score int
}

type wordScores []wordScore

func (this wordScores) Len() int {
	return len(this)
}

func (this wordScores) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this wordScores) Less(i, j int) bool {
	return this[i].score > this[j].score
}

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	wordList, _ := os.Open(*wordListPath)
	reader := bufio.NewReader(wordList)

	words := make([]string, 0)

	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}

		word := string(line)
		if validWord(word) {
			words = append(words, word)
		}
	}

	root := newTrieNode(nil)
	for _, word := range words {
		root.insert(word)
	}

	scores := wordScores(make([]wordScore, 0))
	for _, word := range words {
		scores = append(scores, wordScore{word: word, score: countWords(word, root)})
	}
	sort.Sort(scores)

	wordChannel := make(chan string)
	doneChannel := make(chan interface{})
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go handleRootSearch(root, &words, wordChannel, doneChannel)
	}

	for _, word := range scores {
		wordChannel <- word.word
	}

	close(wordChannel)
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		<-doneChannel
	}
}
