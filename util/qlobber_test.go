package util

import (
	_ "fmt"
	. "gopkg.in/check.v1"
	"os"
	"testing"
)

func Test(t *testing.T) { TestingT(t) }

type QlobberSuite struct{}

var _ = Suite(&QlobberSuite{})

func (s *QlobberSuite) TestQlobber(c *C) {
	q := NewQlobber()
	q.Add("/debug/chobie", "a")
	q.Add("/+/chobie", "b")
	q.Add("/#", "c")

	r := q.Match("/debug/chobie")
	c.Assert(len(r), Equals, 3)
	c.Assert(r[0], Equals, "c") // NOTE: Don't care order
	c.Assert(r[1], Equals, "a")
	c.Assert(r[2], Equals, "b")

	q.Dump(os.Stdout)
}

func (s *QlobberSuite) BenchmarkQlobber(c *C) {
	q := NewQlobber()
	q.Add("/debug/chobie", "a")
	q.Add("/+/chobie", "b")
	q.Add("/#", "c")

	for i := 0; i < c.N; i++ {
		q.Match("/debug/chobie")
	}
}
