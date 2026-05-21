package managed

import "log/slog"

func (s *Service) sysLog() *slog.Logger {
	if s != nil && s.log != nil {
		return s.log
	}
	return slog.Default()
}
