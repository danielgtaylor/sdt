package sdt

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type Test struct {
	Name     string                 `json:"name" yaml:"name"`
	Input    map[string]interface{} `json:"input" yaml:"input"`
	Errors   []string               `json:"errors" yaml:"errors"`
	Expected interface{}            `json:"expected" yaml:"expected"`
}

func (test *Test) ErrorsFail(t testing.TB, errs []error) bool {
	if len(errs) > 0 {
		if len(test.Errors) > 0 {
			// We expected errors. Match them!
		outer:
			for _, expected := range test.Errors {
				for _, actual := range errs {
					if strings.Contains(actual.Error(), expected) {
						continue outer
					}
				}
				t.Error(fmt.Errorf("expected '%s' but found %v", expected, errs))
			}

			return true
		}

		t.Error(errs)
		return true
	}

	return false
}

type Fixture struct {
	Name     string   `json:"-"`
	Document Document `json:"document" yaml:"document"`
	Tests    []Test   `json:"tests" yaml:"tests"`
}

func getFixtures(t testing.TB) []*Fixture {
	files, err := os.ReadDir("fixtures")
	if err != nil {
		t.Error(err)
	}

	fixtures := []*Fixture{}
	for _, file := range files {
		filename, _ := filepath.Abs(path.Join("fixtures", file.Name()))
		if strings.HasSuffix(filename, ".yaml") {
			b, err := ioutil.ReadFile(filename)
			if err != nil {
				t.Error(err)
				return nil
			}

			var f *Fixture
			if err := yaml.Unmarshal(b, &f); err != nil {
				t.Error(err)
				return nil
			}
			f.Name = file.Name()
			f.Document.Filename = filename + "#/document"

			fixtures = append(fixtures, f)
		}
	}

	return fixtures
}

func TestFixtures(t *testing.T) {
	for _, f := range getFixtures(t) {
		for i, test := range f.Tests {
			t.Run(fmt.Sprintf("%s-%d-%s", f.Name, i, test.Name), func(t *testing.T) {
				_, errs := f.Document.ValidateTemplate()
				if test.ErrorsFail(t, errs) {
					return
				}

				err := f.Document.ValidateInput(test.Input)
				if err != nil && test.ErrorsFail(t, []error{err}) {
					return
				}

				out, errs := f.Document.Render(test.Input)
				if test.ErrorsFail(t, errs) {
					return
				}

				err = f.Document.ValidateOutput(out)
				if err != nil && test.ErrorsFail(t, []error{err}) {
					return
				}

				// Round trip to normalize all numbers in the output to make writing
				// test expectations easier in YAML.
				tmp, _ := yaml.Marshal(out)
				yaml.Unmarshal(tmp, &out)

				assert.EqualValues(t, test.Expected, out)
			})
		}
	}
}

func BenchmarkFixtures(b *testing.B) {
	for _, f := range getFixtures(b) {
		for i, test := range f.Tests {
			if test.Errors != nil {
				continue
			}

			b.Run(fmt.Sprintf("LoadSchemas-%s-%d-%s", f.Name, i, test.Name), func(b *testing.B) {
				for j := 0; j < b.N; j++ {
					f.Document.inputSchema = nil
					f.Document.outputSchema = nil
					f.Document.LoadSchemas()
				}
			})
			b.Run(fmt.Sprintf("CheckTemplate-%s-%d-%s", f.Name, i, test.Name), func(b *testing.B) {
				for j := 0; j < b.N; j++ {
					_, errs := f.Document.ValidateTemplate()
					if test.ErrorsFail(b, errs) {
						return
					}
				}
			})
			b.Run(fmt.Sprintf("CheckInput-%s-%d-%s", f.Name, i, test.Name), func(b *testing.B) {
				for j := 0; j < b.N; j++ {
					err := f.Document.ValidateInput(test.Input)
					if err != nil && test.ErrorsFail(b, []error{err}) {
						return
					}
				}
			})
			var out interface{}
			b.Run(fmt.Sprintf("Render-%s-%d-%s", f.Name, i, test.Name), func(b *testing.B) {
				var errs []error
				for j := 0; j < b.N; j++ {
					out, errs = f.Document.Render(test.Input)
					if test.ErrorsFail(b, errs) {
						return
					}
				}
			})
			b.Run(fmt.Sprintf("CheckOutput-%s-%d-%s", f.Name, i, test.Name), func(b *testing.B) {
				for j := 0; j < b.N; j++ {
					err := f.Document.ValidateOutput(out)
					if err != nil && test.ErrorsFail(b, []error{err}) {
						return
					}
				}
			})
		}
	}
}
