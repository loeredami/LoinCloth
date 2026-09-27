package main

import (
	"strconv"
	"testing"

	"github.com/loeredami/ungo"
)

var benchmarkWorkspace *Workspace
var benchmarkWorkspaceCount int

func benchmarkWorkspaceCollections(count int) (*ungo.LinkedList[*Workspace], []*Workspace) {
	list := ungo.NewLinkedList[*Workspace]()
	slice := make([]*Workspace, count)
	for i := 0; i < count; i++ {
		workspace := &Workspace{name: string(rune(i + 1))}
		list.Add(workspace)
		slice[i] = workspace
	}
	return list, slice
}

func BenchmarkWorkspaceIndexedAccess(b *testing.B) {
	for _, count := range []int{1, 4, 16, 64} {
		list, slice := benchmarkWorkspaceCollections(count)
		index := count - 1
		b.Run("LinkedList/last/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				value := list.Get(index)
				if value.HasValue() {
					benchmarkWorkspace = value.Value()
				}
			}
		})
		b.Run("Slice/last/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkWorkspace = slice[index]
			}
		})
	}
}

func BenchmarkWorkspaceTraversal(b *testing.B) {
	for _, count := range []int{1, 4, 16, 64} {
		list, slice := benchmarkWorkspaceCollections(count)
		b.Run("LinkedList/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				total := 0
				list.ForEach(func(_ int, workspace *Workspace) {
					if workspace != nil {
						total++
					}
				})
				benchmarkWorkspaceCount = total
			}
		})
		b.Run("Slice/"+strconv.Itoa(count), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				total := 0
				for _, workspace := range slice {
					if workspace != nil {
						total++
					}
				}
				benchmarkWorkspaceCount = total
			}
		})
	}
}
