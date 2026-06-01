package skiplist

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
)

func newIntStr(tb testing.TB, seed map[int]string) *List[int, string] {
	tb.Helper()
	l := New[int, string]()
	for k, v := range seed {
		l.Set(k, v)
	}
	return l
}

func assertGet(tb testing.TB, l *List[int, string], key int, wantV string, wantOK bool) {
	tb.Helper()
	gotV, gotOK := l.Get(key)
	if gotOK != wantOK || gotV != wantV {
		tb.Fatalf("Get(%d) = (%q, %t); want (%q, %t)", key, gotV, gotOK, wantV, wantOK)
	}
}

func assertLen(tb testing.TB, l *List[int, string], want int) {
	tb.Helper()
	if got := l.Len(); got != want {
		tb.Fatalf("Len() = %d; want %d", got, want)
	}
}

func collectKeys(tb testing.TB, l *List[int, string]) []int {
	tb.Helper()
	var got []int
	for k := range l.All() {
		got = append(got, k)
	}
	return got
}

func assertSorted(tb testing.TB, xs []int) {
	tb.Helper()
	for i := 1; i < len(xs); i++ {
		if xs[i] <= xs[i-1] {
			tb.Fatalf("keys not strictly ascending at %d: %v", i, xs)
		}
	}
}

// scriptedRand returns a deterministic rand.Float64 replacement and fails if a
// test consumes more values than it declared.
func scriptedRand(tb testing.TB, values ...float64) func() float64 {
	tb.Helper()
	var calls int
	return func() float64 {
		tb.Helper()
		if calls >= len(values) {
			tb.Fatalf("rand called %d times; only %d values scripted", calls+1, len(values))
		}
		v := values[calls]
		calls++
		return v
	}
}

func assertPanics(tb testing.TB, fn func()) {
	tb.Helper()
	defer func() {
		if recover() == nil {
			tb.Fatal("expected panic")
		}
	}()
	fn()
}

func ExampleList() {
	var l List[int, string]
	l.Set(3, "three")
	l.Set(1, "one")
	l.Set(2, "two")
	l.Set(2, "TWO")

	value, ok := l.Get(2)
	fmt.Println(value, ok)
	fmt.Println(l.Len())

	minK, minV, _ := l.Min()
	maxK, maxV, _ := l.Max()
	fmt.Printf("min=%d:%s max=%d:%s\n", minK, minV, maxK, maxV)

	l.Delete(1)
	fmt.Println(l.String())

	// Output:
	// TWO true
	// 3
	// min=1:one max=3:three
	// skiplist[2:TWO 3:three]
}

func ExampleList_All() {
	l := New[int, string]()
	l.Set(3, "three")
	l.Set(1, "one")
	l.Set(2, "two")

	for key, value := range l.All() {
		fmt.Printf("%d=%s\n", key, value)
	}

	// Output:
	// 1=one
	// 2=two
	// 3=three
}

func TestRandomTopLevelUsesInjectedRand(t *testing.T) {
	t.Parallel()

	values := []float64{0.1, 0.2, 0.3}
	var calls int
	l := New[int, string](WithRandFloat(func() float64 {
		v := values[calls]
		calls++
		return v
	}))

	if got, want := l.randomTopLevel(), 2; got != want {
		t.Fatalf("randomTopLevel() = %d; want %d", got, want)
	}
	if got, want := calls, 3; got != want {
		t.Fatalf("rand calls = %d; want %d", got, want)
	}
}

func TestNewUsesConfiguredLevelOptions(t *testing.T) {
	t.Parallel()

	var calls int
	l := New[int, string](
		WithMaxLevel(2),
		WithProbability(0.75),
		WithRandFloat(func() float64 {
			calls++
			return 0.1
		}),
	)

	if got, want := len(l.head.forward), 2; got != want {
		t.Fatalf("head forward length = %d; want %d", got, want)
	}
	if got, want := l.randomTopLevel(), 1; got != want {
		t.Fatalf("randomTopLevel() = %d; want %d", got, want)
	}
	if got, want := calls, 1; got != want {
		t.Fatalf("rand calls = %d; want %d", got, want)
	}
}

func TestProbabilityZeroDoesNotCallRand(t *testing.T) {
	t.Parallel()

	l := New[int, string](
		WithProbability(0),
		WithRandFloat(func() float64 {
			t.Fatal("rand should not be called when probability is zero")
			return 0
		}),
	)

	if got, want := l.randomTopLevel(), 0; got != want {
		t.Fatalf("randomTopLevel() = %d; want %d", got, want)
	}
}

func TestOptionValidation(t *testing.T) {
	t.Parallel()

	assertPanics(t, func() { New[int, string](WithMaxLevel(0)) })
	assertPanics(t, func() { New[int, string](WithProbability(-0.1)) })
	assertPanics(t, func() { New[int, string](WithProbability(1)) })

	l := New[int, string](WithRandFloat(nil))
	l.Set(1, "a")
	assertGet(t, l, 1, "a", true)

	l = New[int, string](WithRand(nil))
	l.Set(2, "b")
	assertGet(t, l, 2, "b", true)
}

func TestZeroValueList(t *testing.T) {
	t.Parallel()

	var l List[int, string]
	assertLen(t, &l, 0)
	assertGet(t, &l, 1, "", false)
	if got := collectKeys(t, &l); len(got) != 0 {
		t.Fatalf("All on zero value = %v; want none", got)
	}
	if got, want := l.String(), "skiplist[]"; got != want {
		t.Fatalf("String() = %q; want %q", got, want)
	}

	l.Set(2, "b")
	l.Set(1, "a")
	assertLen(t, &l, 2)
	assertGet(t, &l, 1, "a", true)
	if k, v, ok := l.Max(); !ok || k != 2 || v != "b" {
		t.Fatalf("Max() = (%d, %q, %t); want (2, %q, true)", k, v, ok, "b")
	}
	if got, want := fmt.Sprint(collectKeys(t, &l)), fmt.Sprint([]int{1, 2}); got != want {
		t.Fatalf("keys = %s; want %s", got, want)
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		seed   map[int]string
		key    int
		wantV  string
		wantOK bool
	}{
		{"empty list", nil, 1, "", false},
		{"present single", map[int]string{1: "a"}, 1, "a", true},
		{"absent below min", map[int]string{5: "a", 9: "b"}, 1, "", false},
		{"absent above max", map[int]string{5: "a", 9: "b"}, 99, "", false},
		{"absent in gap", map[int]string{5: "a", 9: "b"}, 7, "", false},
		{"present negative key", map[int]string{-3: "neg"}, -3, "neg", true},
		{"present zero key", map[int]string{0: "zero"}, 0, "zero", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := newIntStr(t, tc.seed)
			assertGet(t, l, tc.key, tc.wantV, tc.wantOK)
		})
	}
}

func TestSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ops  []struct {
			k int
			v string
		}
		checkKey int
		wantV    string
		wantLen  int
	}{
		{
			name: "insert one",
			ops: []struct {
				k int
				v string
			}{{1, "a"}},
			checkKey: 1, wantV: "a", wantLen: 1,
		},
		{
			name: "overwrite keeps len",
			ops: []struct {
				k int
				v string
			}{{1, "a"}, {1, "b"}},
			checkKey: 1, wantV: "b", wantLen: 1,
		},
		{
			name: "insert ascending",
			ops: []struct {
				k int
				v string
			}{{1, "a"}, {2, "b"}, {3, "c"}},
			checkKey: 3, wantV: "c", wantLen: 3,
		},
		{
			name: "insert descending",
			ops: []struct {
				k int
				v string
			}{{3, "c"}, {2, "b"}, {1, "a"}},
			checkKey: 1, wantV: "a", wantLen: 3,
		},
		{
			name: "insert shuffled with dup overwrite",
			ops: []struct {
				k int
				v string
			}{{5, "e"}, {1, "a"}, {3, "c"}, {3, "C"}},
			checkKey: 3, wantV: "C", wantLen: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := New[int, string]()
			for _, op := range tc.ops {
				l.Set(op.k, op.v)
			}
			assertGet(t, l, tc.checkKey, tc.wantV, true)
			assertLen(t, l, tc.wantLen)
			assertSorted(t, collectKeys(t, l))
		})
	}
}

func TestFloatNaNKeyUsesCmpOrdering(t *testing.T) {
	t.Parallel()

	l := New[float64, string](WithRandFloat(func() float64 { return 1 }))
	l.Set(1, "one")
	l.Set(math.NaN(), "nan")

	if got, ok := l.Get(math.NaN()); !ok || got != "nan" {
		t.Fatalf("Get(NaN) = (%q, %t); want (%q, true)", got, ok, "nan")
	}

	var keys []float64
	for k := range l.All() {
		keys = append(keys, k)
	}
	if len(keys) != 2 || !math.IsNaN(keys[0]) || keys[1] != 1 {
		t.Fatalf("Range keys = %v; want [NaN 1]", keys)
	}
}

func TestFloatNaNKeyOverwriteAndDelete(t *testing.T) {
	t.Parallel()

	l := New[float64, string](WithRandFloat(func() float64 { return 1 }))
	nan := math.NaN()

	l.Set(nan, "old")
	l.Set(nan, "new")
	if got, ok := l.Get(math.NaN()); !ok || got != "new" {
		t.Fatalf("Get(NaN) = (%q, %t); want (%q, true)", got, ok, "new")
	}
	if got := l.Len(); got != 1 {
		t.Fatalf("Len() = %d; want 1", got)
	}

	if !l.Delete(math.NaN()) {
		t.Fatal("Delete(NaN) = false; want true")
	}
	if got, ok := l.Get(math.NaN()); ok || got != "" {
		t.Fatalf("Get(NaN) after delete = (%q, %t); want (%q, false)", got, ok, "")
	}
	if got := l.Len(); got != 0 {
		t.Fatalf("Len() after delete = %d; want 0", got)
	}
}

func TestMaxLevelOneBehavesLikeSortedList(t *testing.T) {
	t.Parallel()

	l := New[int, string](
		WithMaxLevel(1),
		WithRandFloat(func() float64 { return 0 }),
	)
	for _, op := range []struct {
		k int
		v string
	}{
		{3, "c"},
		{1, "a"},
		{2, "b"},
		{2, "B"},
	} {
		l.Set(op.k, op.v)
	}

	if got, want := l.topLevel, 0; got != want {
		t.Fatalf("topLevel = %d; want %d", got, want)
	}
	assertLen(t, l, 3)
	assertGet(t, l, 2, "B", true)
	if got, want := fmt.Sprint(collectKeys(t, l)), fmt.Sprint([]int{1, 2, 3}); got != want {
		t.Fatalf("keys = %s; want %s", got, want)
	}

	if !l.Delete(1) {
		t.Fatal("Delete(1) = false; want true")
	}
	if got, want := fmt.Sprint(collectKeys(t, l)), fmt.Sprint([]int{2, 3}); got != want {
		t.Fatalf("keys after delete = %s; want %s", got, want)
	}
}

func TestTallNodeInsertionCapsAtMaxTopLevel(t *testing.T) {
	t.Parallel()

	l := New[int, string](
		WithMaxLevel(4),
		WithRandFloat(scriptedRand(t, 0, 0, 0)),
	)
	l.Set(10, "x")

	if got, want := l.topLevel, 3; got != want {
		t.Fatalf("topLevel = %d; want %d", got, want)
	}
	if got, want := len(l.head.forward), 4; got != want {
		t.Fatalf("head forward length = %d; want %d", got, want)
	}
	assertGet(t, l, 10, "x", true)
}

func TestMaxTracksTail(t *testing.T) {
	t.Parallel()

	l := New[int, string]()
	if k, v, ok := l.Max(); ok || k != 0 || v != "" {
		t.Fatalf("Max() on empty = (%d, %q, %t); want (0, %q, false)", k, v, ok, "")
	}

	l.Set(2, "b")
	l.Set(1, "a")
	l.Set(3, "c")
	l.Set(3, "C")
	if k, v, ok := l.Max(); !ok || k != 3 || v != "C" {
		t.Fatalf("Max() = (%d, %q, %t); want (3, %q, true)", k, v, ok, "C")
	}

	if !l.Delete(3) {
		t.Fatal("Delete(3) = false; want true")
	}
	if k, v, ok := l.Max(); !ok || k != 2 || v != "b" {
		t.Fatalf("Max() after deleting tail = (%d, %q, %t); want (2, %q, true)", k, v, ok, "b")
	}

	if !l.Delete(2) || !l.Delete(1) {
		t.Fatal("Delete remaining keys = false; want true")
	}
	if k, v, ok := l.Max(); ok || k != 0 || v != "" {
		t.Fatalf("Max() after deleting all = (%d, %q, %t); want (0, %q, false)", k, v, ok, "")
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		seed        map[int]string
		delKey      int
		wantDeleted bool
		wantLen     int
	}{
		{"from empty", nil, 1, false, 0},
		{"only element", map[int]string{1: "a"}, 1, true, 0},
		{"head element", map[int]string{1: "a", 2: "b", 3: "c"}, 1, true, 2},
		{"tail element", map[int]string{1: "a", 2: "b", 3: "c"}, 3, true, 2},
		{"middle element", map[int]string{1: "a", 2: "b", 3: "c"}, 2, true, 2},
		{"absent in populated", map[int]string{1: "a", 2: "b"}, 99, false, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := newIntStr(t, tc.seed)
			if got := l.Delete(tc.delKey); got != tc.wantDeleted {
				t.Fatalf("Delete(%d) = %t; want %t", tc.delKey, got, tc.wantDeleted)
			}
			assertLen(t, l, tc.wantLen)
			assertGet(t, l, tc.delKey, "", false) // gone (or never there)
			assertSorted(t, collectKeys(t, l))
		})
	}
}

func TestDeleteThenReinsert(t *testing.T) {
	t.Parallel()
	l := newIntStr(t, map[int]string{1: "a", 2: "b", 3: "c"})

	if !l.Delete(2) {
		t.Fatal("Delete(2) = false; want true")
	}
	assertGet(t, l, 2, "", false)

	l.Set(2, "B")
	assertGet(t, l, 2, "B", true)
	assertLen(t, l, 3)
	if got := collectKeys(t, l); len(got) != 3 {
		t.Fatalf("keys after reinsert = %v; want 3 keys", got)
	}
}

func TestDeleteShrinksTopLevel(t *testing.T) {
	t.Parallel()

	l := New[int, string](
		WithMaxLevel(4),
		WithRandFloat(scriptedRand(t,
			0, 0, 0, // key 1 reaches top level 3.
			1, // key 2 stays at level 0.
			1, // key 3 stays at level 0.
		)),
	)
	l.Set(1, "a")
	l.Set(2, "b")
	l.Set(3, "c")

	if got, want := l.topLevel, 3; got != want {
		t.Fatalf("topLevel before delete = %d; want %d", got, want)
	}
	if !l.Delete(1) {
		t.Fatal("Delete(1) = false; want true")
	}
	if got, want := l.topLevel, 0; got != want {
		t.Fatalf("topLevel after delete = %d; want %d", got, want)
	}
	if got, want := fmt.Sprint(collectKeys(t, l)), fmt.Sprint([]int{2, 3}); got != want {
		t.Fatalf("keys after delete = %s; want %s", got, want)
	}
}

// --- Iteration -----------------------------------------------------------

func TestAll(t *testing.T) {
	t.Parallel()

	t.Run("ascending order over shuffled inserts", func(t *testing.T) {
		t.Parallel()
		l := newIntStr(t, map[int]string{4: "d", 1: "a", 3: "c", 2: "b"})
		got := collectKeys(t, l)
		want := []int{1, 2, 3, 4}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Fatalf("All keys = %v; want %v", got, want)
		}
	})

	t.Run("empty list yields nothing", func(t *testing.T) {
		t.Parallel()
		l := New[int, string]()
		if got := collectKeys(t, l); len(got) != 0 {
			t.Fatalf("All over empty = %v; want none", got)
		}
	})

	t.Run("early break stops iteration", func(t *testing.T) {
		t.Parallel()
		l := newIntStr(t, map[int]string{1: "a", 2: "b", 3: "c", 4: "d"})
		var seen []int
		for k := range l.All() {
			seen = append(seen, k)
			if k >= 2 {
				break
			}
		}
		want := []int{1, 2}
		if fmt.Sprint(seen) != fmt.Sprint(want) {
			t.Fatalf("early-break keys = %v; want %v", seen, want)
		}
	})
}

func TestAllYieldsKeyValuePairsAfterOverwrite(t *testing.T) {
	t.Parallel()

	l := New[int, string]()
	l.Set(2, "old")
	l.Set(1, "a")
	l.Set(2, "b")

	var got []string
	for k, v := range l.All() {
		got = append(got, fmt.Sprintf("%d:%s", k, v))
	}
	want := []string{"1:a", "2:b"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("All pairs = %v; want %v", got, want)
	}
}

func TestAllCanBeIteratedRepeatedly(t *testing.T) {
	t.Parallel()

	l := newIntStr(t, map[int]string{3: "c", 1: "a", 2: "b"})
	first := collectKeys(t, l)
	second := collectKeys(t, l)

	want := []int{1, 2, 3}
	if fmt.Sprint(first) != fmt.Sprint(want) || fmt.Sprint(second) != fmt.Sprint(want) {
		t.Fatalf("repeated All keys = %v then %v; want %v both times", first, second, want)
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	if got, want := New[int, string]().String(), "skiplist[]"; got != want {
		t.Fatalf("String() = %q; want %q", got, want)
	}

	l := newIntStr(t, map[int]string{3: "c", 1: "a", 2: "b"})
	if got, want := l.String(), "skiplist[1:a 2:b 3:c]"; got != want {
		t.Fatalf("String() = %q; want %q", got, want)
	}
}

func TestRange(t *testing.T) {
	t.Parallel()

	l := newIntStr(t, map[int]string{1: "a", 2: "b", 3: "c"})
	var seen []int
	l.Range(func(k int, _ string) bool {
		seen = append(seen, k)
		return k < 2
	})

	want := []int{1, 2}
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Fatalf("Range keys = %v; want %v", seen, want)
	}
}

func TestMinMax(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		seed               map[int]string
		wantMinK, wantMaxK int
		wantMinV, wantMaxV string
		wantOK             bool
	}{
		{"empty", nil, 0, 0, "", "", false},
		{"single", map[int]string{7: "g"}, 7, 7, "g", "g", true},
		{"many", map[int]string{3: "c", 1: "a", 9: "i", -2: "neg"}, -2, 9, "neg", "i", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := newIntStr(t, tc.seed)

			minK, minV, minOK := l.Min()
			if minOK != tc.wantOK || (tc.wantOK && (minK != tc.wantMinK || minV != tc.wantMinV)) {
				t.Fatalf("Min() = (%d, %q, %t); want (%d, %q, %t)",
					minK, minV, minOK, tc.wantMinK, tc.wantMinV, tc.wantOK)
			}

			maxK, maxV, maxOK := l.Max()
			if maxOK != tc.wantOK || (tc.wantOK && (maxK != tc.wantMaxK || maxV != tc.wantMaxV)) {
				t.Fatalf("Max() = (%d, %q, %t); want (%d, %q, %t)",
					maxK, maxV, maxOK, tc.wantMaxK, tc.wantMaxV, tc.wantOK)
			}
		})
	}
}

func TestAgainstMapModel(t *testing.T) {
	t.Parallel()

	const ops = 5000
	const keySpace = 200

	rng := rand.New(rand.NewSource(1))
	l := New[int, string]()
	model := map[int]string{}

	for i := range ops {
		k := rng.Intn(keySpace)
		switch rng.Intn(3) {
		case 0: // Set
			v := fmt.Sprintf("v%d", i)
			l.Set(k, v)
			model[k] = v
		case 1: // Delete
			wantDeleted := false
			if _, ok := model[k]; ok {
				wantDeleted = true
			}
			if got := l.Delete(k); got != wantDeleted {
				t.Fatalf("op %d: Delete(%d) = %t; want %t", i, k, got, wantDeleted)
			}
			delete(model, k)
		case 2: // Get
			wantV, wantOK := model[k]
			gotV, gotOK := l.Get(k)
			if gotOK != wantOK || gotV != wantV {
				t.Fatalf("op %d: Get(%d) = (%q,%t); want (%q,%t)", i, k, gotV, gotOK, wantV, wantOK)
			}
		}

		if got, want := l.Len(), len(model); got != want {
			t.Fatalf("op %d: Len() = %d; want %d", i, got, want)
		}
	}

	// Final structural check: iteration order must equal sorted model keys.
	wantKeys := make([]int, 0, len(model))
	for k := range model {
		wantKeys = append(wantKeys, k)
	}
	sort.Ints(wantKeys)

	gotKeys := collectKeys(t, l)
	assertSorted(t, gotKeys)
	if fmt.Sprint(gotKeys) != fmt.Sprint(wantKeys) {
		t.Fatalf("final keys mismatch:\n got  %v\n want %v", gotKeys, wantKeys)
	}
}
