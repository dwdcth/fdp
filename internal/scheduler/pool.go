package scheduler

import (
	"sync"

	"docker_aria2c/internal/downloader"
)

func Run(tasks []downloader.Task, workers int, fn func(task downloader.Task) error) error {
	if len(tasks) == 0 {
		return nil
	}
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan downloader.Task)
	errCh := make(chan error, len(tasks))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if err := fn(task); err != nil {
					errCh <- err
				}
			}
		}()
	}

	for _, task := range tasks {
		jobs <- task
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}
