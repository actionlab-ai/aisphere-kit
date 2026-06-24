package starter

import "errors"

type CleanupStack struct{ funcs []func() error }

func (s *CleanupStack) Add(fn func() error) {
	if fn != nil {
		s.funcs = append(s.funcs, fn)
	}
}
func (s *CleanupStack) Close() error {
	var errs []error
	for i := len(s.funcs) - 1; i >= 0; i-- {
		if err := s.funcs[i](); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
