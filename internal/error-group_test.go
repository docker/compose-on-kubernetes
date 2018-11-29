package internal

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	var noerr error
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	// 0
	assert.Equal(t, GroupErrors(noerr), nil)
	assert.Equal(t, GroupErrors(noerr, noerr), nil)

	// 1
	assert.Equal(t, GroupErrors(err1), err1)
	assert.Equal(t, GroupErrors(noerr, err1), err1)
	assert.Equal(t, GroupErrors(err1, noerr), err1)

	// 2
	const (
		err12 = `multiple errors: "error 1", "error 2"`
	)
	assert.Equal(t, err12, GroupErrors(noerr, err1, err2).Error())
	assert.Equal(t, err12, GroupErrors(err1, noerr, err2).Error())

	// infinity (for a chemist).
	const (
		err1212 = `multiple errors: "error 1", "error 2", "error 1", "error 2"`
	)
	assert.Equal(t, err1212,
		GroupErrors(noerr, err1, err2, noerr, err1, err2).Error())
	assert.Equal(t, err1212,
		GroupErrors(noerr, err1, err2, noerr, GroupErrors(err1, err2)).Error())
	assert.Equal(t, err1212,
		GroupErrors(noerr, GroupErrors(err1, err2), noerr, GroupErrors(err1, err2)).Error())
}
