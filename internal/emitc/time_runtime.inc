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

static const char *uli_time_format_rfc3339_loc(int64_t unix_nano, const char *loc) {
	if (!loc || !loc[0] || strcmp(loc, "UTC") == 0) {
		return uli_time_format_rfc3339(unix_nano);
	}
	if (strcmp(loc, "Local") != 0) {
		return uli_time_format_rfc3339(unix_nano);
	}
	time_t sec = (time_t)(unix_nano / uli_time_nsec_per_sec);
	int64_t nsec = unix_nano % uli_time_nsec_per_sec;
	if (nsec < 0) {
		sec -= 1;
		nsec += uli_time_nsec_per_sec;
	}
	struct tm tm;
	if (localtime_r(&sec, &tm) == NULL) {
		return "";
	}
	long off = 0;
#if defined(__linux__) || defined(__APPLE__)
	off = tm.tm_gmtoff;
#else
	{
		struct tm gmt;
		if (gmtime_r(&sec, &gmt) == NULL) return "";
		off = (long)(mktime(&tm) - mktime(&gmt));
	}
#endif
	int sign = off >= 0 ? 1 : -1;
	long aoff = off * sign;
	int oh = (int)(aoff / 3600);
	int om = (int)((aoff % 3600) / 60);
	char buf[80];
	int n = snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d.%09lld%c%02d:%02d",
		tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
		tm.tm_hour, tm.tm_min, tm.tm_sec, (long long)nsec,
		sign >= 0 ? '+' : '-', oh, om);
	if (n <= 0 || (size_t)n >= sizeof(buf)) return "";
	char *out = (char *)uli_arena_alloc((size_t)n + 1);
	memcpy(out, buf, (size_t)n + 1);
	return out;
}

typedef struct { int64_t value; void *err; } uli_time_parse_result;
typedef struct { const char *message; } uli_time_error;

static void *uli_time_error_new(const char *message) {
	uli_time_error *err = (uli_time_error *)uli_alloc(sizeof(*err));
	err->message = message ? message : "time parse error";
	return err;
}

static int uli_time_parse_digits(const char *s, int n, int *out) {
	int v = 0;
	for (int i = 0; i < n; i++) {
		if (s[i] < '0' || s[i] > '9') return -1;
		v = v * 10 + (s[i] - '0');
	}
	*out = v;
	return 0;
}

static uli_time_parse_result uli_time_parse_rfc3339(const char *s) {
	if (!s || strlen(s) < 20) return (uli_time_parse_result){ 0, uli_time_error_new("invalid RFC3339 time") };
	int year, month, day, hour, min, sec;
	if (uli_time_parse_digits(s, 4, &year) || s[4] != '-' ||
	    uli_time_parse_digits(s + 5, 2, &month) || s[7] != '-' ||
	    uli_time_parse_digits(s + 8, 2, &day) || (s[10] != 'T' && s[10] != 't') ||
	    uli_time_parse_digits(s + 11, 2, &hour) || s[13] != ':' ||
	    uli_time_parse_digits(s + 14, 2, &min) || s[16] != ':' ||
	    uli_time_parse_digits(s + 17, 2, &sec)) {
		return (uli_time_parse_result){ 0, uli_time_error_new("invalid RFC3339 time") };
	}
	const char *p = s + 19;
	int64_t nsec = 0;
	if (*p == '.') {
		p++;
		int digits = 0;
		while (*p >= '0' && *p <= '9' && digits < 9) {
			nsec = nsec * 10 + (*p - '0');
			p++;
			digits++;
		}
		while (digits > 0 && digits < 9) { nsec *= 10; digits++; }
		while (*p >= '0' && *p <= '9') p++;
	}
	int off_sec = 0;
	if (*p == 'Z' || *p == 'z') {
		p++;
	} else if (*p == '+' || *p == '-') {
		int sign = (*p == '+') ? 1 : -1;
		p++;
		int oh, om;
		if (uli_time_parse_digits(p, 2, &oh) || p[2] != ':' || uli_time_parse_digits(p + 3, 2, &om)) {
			return (uli_time_parse_result){ 0, uli_time_error_new("invalid RFC3339 time") };
		}
		off_sec = sign * (oh * 3600 + om * 60);
		p += 5;
	} else {
		return (uli_time_parse_result){ 0, uli_time_error_new("invalid RFC3339 time") };
	}
	if (*p != '\0') return (uli_time_parse_result){ 0, uli_time_error_new("invalid RFC3339 time") };
	if (month < 1 || month > 12 || day < 1 || day > 31 || hour > 23 || min > 59 || sec > 60) {
		return (uli_time_parse_result){ 0, uli_time_error_new("invalid RFC3339 time") };
	}
	struct tm tm;
	memset(&tm, 0, sizeof tm);
	tm.tm_year = year - 1900;
	tm.tm_mon = month - 1;
	tm.tm_mday = day;
	tm.tm_hour = hour;
	tm.tm_min = min;
	tm.tm_sec = sec;
#if defined(_GNU_SOURCE) || defined(__USE_BSD)
	tm.tm_gmtoff = 0;
#endif
	time_t sec_t = timegm(&tm);
	if (sec_t == (time_t)-1 && (year < 1970)) {
		/* timegm may fail on some platforms for edge values; still attempt. */
	}
	int64_t unix_nano = (int64_t)sec_t * uli_time_nsec_per_sec + nsec - (int64_t)off_sec * uli_time_nsec_per_sec;
	return (uli_time_parse_result){ unix_nano, NULL };
}
