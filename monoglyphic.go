package main

import (
	"bufio"
	"container/list"
	"fmt"
	"os"
	"runtime"
)

var _ = fmt.Println

const (
	wordListPath = "/usr/share/dict/words"
)

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

func (this *trieNode) findUnconflictedTerminals(used letterSet, result *list.List) {
	if this.used.conflictsWith(used) {
		return
	}

	if this.terminal {
		result.PushBack(this)
	}

	for _, next := range this.next {
		next.findUnconflictedTerminals(used, result)
	}
}

func toIndex(input uint8) uint {
	return uint(input - 'a')
}

func validWord(input string) bool {
	// TODO(gdk): Find a better wordlist.
	if len(input) == 1 && input != "a" {
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

func bump(input string) {}

var recordLength = 0
var recordPartial = ""

func augmentPartial(partial string, root *trieNode, depth int) {
	for i := len(partial); i >= 0; i-- {
		prefix, suffix := partial[:i], partial[i:]
		prefixSet := toLetterSet(prefix)
		suffixNode := root.walk(suffix)

		if suffixNode != nil {
			validSuffixes := list.New()
			suffixNode.findUnconflictedTerminals(prefixSet, validSuffixes)

			for i := validSuffixes.Front(); i != nil; i = i.Next() {
				validSuffix := i.Value.(*trieNode).partial
				if len(validSuffix) <= len(suffix) {
					continue
				}

				length := countWords(prefix+validSuffix, root)
				if length > recordLength {
					recordLength = length
					recordPartial = prefix + validSuffix
					fmt.Println(recordPartial, recordLength)
				}

				newPartial := prefix + validSuffix
				augmentPartial(newPartial, root, depth+1)
			}
		}
	}
}

func handleRootSearch(root *trieNode, words chan string, done chan interface{}) {
	for word := range words {
		augmentPartial(word, root, 0)
	}
	<-done
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	wordList, _ := os.Open(wordListPath)
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

	// words := []string{"ambling", "go"}

	root := newTrieNode(nil)
	for _, word := range words {
		root.insert(word)
	}

	wordChannel := make(chan string)
	doneChannel := make(chan interface{})
	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		go handleRootSearch(root, wordChannel, doneChannel)
	}

	for _, word := range words {
		if len(word) > 5 {
			wordChannel <- word
		}
	}

	for i := 0; i < runtime.GOMAXPROCS(0); i++ {
		<-doneChannel
	}
}
