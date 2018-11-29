package wait

import (
	"fmt"
	"time"
)

type condition func() (bool, error)

// For is a trivial polling loop.
func For(maxCount int, condition condition) error {
	for i := 0; i < maxCount; i++ {
		ok, err := condition()
		if err != nil {
			fmt.Println(err)
		} else if ok {
			return nil
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("condition not met after %d retries", maxCount)
}
