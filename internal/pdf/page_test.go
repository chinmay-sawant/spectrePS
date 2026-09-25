package pdf

import (
	"slices"
	"testing"
)

func TestPageContentNums(t *testing.T) {
	t.Parallel()
	checkJoinedContentNums(t)
	checkSingleContentNum(t)
	checkNoContentNums(t)
	checkContentNumRange(t)
}

func checkJoinedContentNums(t *testing.T) {
	t.Helper()
	file := mustOpen(t, joinedPage(t))
	nums, err := file.PageContentNums(0)
	if err != nil {
		t.Fatal(err)
	}
	// joinedPage stores its two content streams as objects 4 and 5.
	want := []int{4, 5}
	if !slices.Equal(nums, want) {
		t.Fatalf("nums %v want %v", nums, want)
	}
}

func checkSingleContentNum(t *testing.T) {
	t.Helper()
	file := mustOpen(t, classicLine(t))
	nums, err := file.PageContentNums(0)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(nums, []int{idContent}) {
		t.Fatalf("nums %v want [%d]", nums, idContent)
	}
}

func checkNoContentNums(t *testing.T) {
	t.Helper()
	doc := newDoc()
	doc.object("<< /Type /Catalog /Pages 2 0 R >>")
	doc.object("<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	doc.object("<< /Type /Page /Parent 2 0 R >>")
	file := mustOpen(t, doc.classic(""))
	nums, err := file.PageContentNums(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(nums) != 0 {
		t.Fatalf("nums %v want none", nums)
	}
}

func checkContentNumRange(t *testing.T) {
	t.Helper()
	file := mustOpen(t, classicLine(t))
	for _, index := range []int{-1, 1} {
		_, err := file.PageContentNums(index)
		wantJob(t, err, opPDF, errRange)
	}
}
