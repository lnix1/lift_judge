package video_feed

/*
#include <fcntl.h>
#include <semaphore.h>
#include <stdlib.h>
#include <sys/stat.h>

// Wrapper to handle C's variadic sem_open for Go
static sem_t* open_sem(const char* name, int oflag, mode_t mode, unsigned int value) {
    return sem_open(name, oflag, mode, value);
}

// Helper to check for SEM_FAILED since it's a complex macro
static int is_sem_failed(sem_t *sem) {
    return sem == SEM_FAILED;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type Semaphore struct {
	ptr *C.sem_t
}

func NewSemaphore(name string) (*Semaphore, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	// Call our wrapper instead of sem_open directly
	// O_CREAT = 64
	ptr, err := C.open_sem(cName, C.int(64), C.mode_t(0666), C.uint(0))
	
	if C.is_sem_failed(ptr) != 0 {
		return nil, fmt.Errorf("sem_open failed: %v", err)
	}

	return &Semaphore{ptr: ptr}, nil
}

func (s *Semaphore) Post() error {
	_, err := C.sem_post(s.ptr)
	return err
}

func (s *Semaphore) Close() error {
	_, err := C.sem_close(s.ptr)
	return err
}
