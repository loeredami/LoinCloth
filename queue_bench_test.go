package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/loeredami/ungo"
)

var benchmarkQueuedCommand string
var benchmarkQueueCount int

func TestQueuePreservesPastedCommandOrder(t *testing.T) {
	queue := ungo.NewQueue[string]()
	commands := []string{"first", "", "third"}
	for _, command := range commands {
		queue.Push(command)
	}
	for i, want := range commands {
		got := queue.Pop()
		if !got.HasValue() {
			t.Fatalf("queue ended before command %d", i)
		}
		if got.Value() != want {
			t.Fatalf("command %d = %q, want %q", i, got.Value(), want)
		}
	}
	if !queue.IsEmpty() || queue.Pop().HasValue() {
		t.Fatal("queue was not empty after all commands were consumed")
	}
}

func BenchmarkCommandBatchQueue(b *testing.B) {
	for _, count := range []int{700, 5000} {
		commands := make([]string, count)
		for i := range commands {
			commands[i] = "command-" + strconv.Itoa(i)
		}
		b.Run("UngoQueue/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				queue := ungo.NewQueue[string]()
				for _, command := range commands {
					queue.Push(command)
				}
				total := 0
				for !queue.IsEmpty() {
					command := queue.Pop()
					if command.HasValue() {
						benchmarkQueuedCommand = command.Value()
						total++
					}
				}
				benchmarkQueueCount = total
			}
		})
		b.Run("SliceCursor/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				queue := make([]string, 0, len(commands))
				queue = append(queue, commands...)
				total := 0
				for cursor := 0; cursor < len(queue); cursor++ {
					benchmarkQueuedCommand = queue[cursor]
					total++
				}
				benchmarkQueueCount = total
			}
		})
		b.Run("StringBuffer/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pending := strings.Join(commands, "\n")
				total := 0
				for pending != "" {
					end := strings.IndexByte(pending, '\n')
					if end < 0 {
						benchmarkQueuedCommand = pending
						pending = ""
					} else {
						benchmarkQueuedCommand = pending[:end]
						pending = pending[end+1:]
					}
					total++
				}
				benchmarkQueueCount = total
			}
		})
	}
}
