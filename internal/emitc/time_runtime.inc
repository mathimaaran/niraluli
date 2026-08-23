/* Niraluli Tamil-0.64: wall clock + duration (Linux). Sleep parks for GC. */
#include <errno.h>
#include <time.h>

enum {
	uli_time_nsec_per_usec = 1000LL,
	uli_time_nsec_per_msec = 1000000LL,
	uli_time_nsec_per_sec = 1000000000LL
};

static int64_t uli_time_now_unix_nano(void) {
	struct timespec ts;
	if (clock_gettime(CLOCK_REALTIME, &ts) != 0) {
		return 0;
	}
	return (int64_t)ts.tv_sec * uli_time_nsec_per_sec + (int64_t)ts.tv_nsec;
}

static void uli_time_sleep(int64_t nsec) {
	if (nsec <= 0) return;
	struct timespec req;
	req.tv_sec = (time_t)(nsec / uli_time_nsec_per_sec);
	req.tv_nsec = (long)(nsec % uli_time_nsec_per_sec);
	char sp;
	for (;;) {
		uli_gc_park_begin(&sp);
		int rc = nanosleep(&req, &req);
		uli_gc_park_end();
		if (rc == 0) return;
		if (errno != EINTR) return;
	}
}

static const char *uli_time_format_rfc3339(int64_t unix_nano) {
	time_t sec = (time_t)(unix_nano / uli_time_nsec_per_sec);
	int64_t nsec = unix_nano % uli_time_nsec_per_sec;
	if (nsec < 0) {
		sec -= 1;
		nsec += uli_time_nsec_per_sec;
	}
	struct tm tm;
	if (gmtime_r(&sec, &tm) == NULL) {
		return "";
	}
	char buf[64];
	int n = snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d.%09lldZ",
		tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
		tm.tm_hour, tm.tm_min, tm.tm_sec, (long long)nsec);
	if (n <= 0 || (size_t)n >= sizeof(buf)) {
		return "";
	}
	char *out = (char *)uli_arena_alloc((size_t)n + 1);
	memcpy(out, buf, (size_t)n + 1);
	return out;
}
