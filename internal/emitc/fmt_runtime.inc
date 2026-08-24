/* Niraluli Tamil-0.65: fmt-style formatting (Linux). */
#include <stdio.h>

static const char *uli_fmt_int(int64_t n) {
	char buf[32];
	int len = snprintf(buf, sizeof(buf), "%lld", (long long)n);
	if (len <= 0 || (size_t)len >= sizeof(buf)) return "";
	char *out = (char *)uli_arena_alloc((size_t)len + 1);
	memcpy(out, buf, (size_t)len + 1);
	return out;
}

static const char *uli_fmt_float(double f) {
	char buf[64];
	int len = snprintf(buf, sizeof(buf), "%g", f);
	if (len <= 0 || (size_t)len >= sizeof(buf)) return "";
	char *out = (char *)uli_arena_alloc((size_t)len + 1);
	memcpy(out, buf, (size_t)len + 1);
	return out;
}

static const char *uli_fmt_bool(int b) {
	return b ? "மெய்" : "பொய்";
}

static const char *uli_fmt_arg(const char **data, int64_t len, int64_t *idx) {
	if (*idx < 0 || *idx >= len) return "%!s(MISSING)";
	const char *s = data ? data[*idx] : NULL;
	(*idx)++;
	return s ? s : "";
}

static const char *uli_fmt_sprintf(const char *format, const char **data, int64_t len) {
	if (!format) format = "";
	size_t total = 0;
	int64_t ai = 0;
	for (const char *p = format; *p; ) {
		if (p[0] == '%' && p[1] == '%') {
			total += 1;
			p += 2;
			continue;
		}
		if (p[0] == '%' && p[1] == 's') {
			const char *arg = uli_fmt_arg(data, len, &ai);
			total += strlen(arg);
			p += 2;
			continue;
		}
		total += 1;
		p += 1;
	}
	char *out = (char *)uli_arena_alloc(total + 1);
	char *w = out;
	ai = 0;
	for (const char *p = format; *p; ) {
		if (p[0] == '%' && p[1] == '%') {
			*w++ = '%';
			p += 2;
			continue;
		}
		if (p[0] == '%' && p[1] == 's') {
			const char *arg = uli_fmt_arg(data, len, &ai);
			size_t n = strlen(arg);
			memcpy(w, arg, n);
			w += n;
			p += 2;
			continue;
		}
		*w++ = *p++;
	}
	*w = 0;
	return out;
}

static const char *uli_fmt_join(const char **data, int64_t len, const char *sep) {
	if (!sep) sep = "";
	if (len <= 0) return "";
	size_t sepn = strlen(sep);
	size_t total = 0;
	for (int64_t i = 0; i < len; i++) {
		const char *s = data && data[i] ? data[i] : "";
		total += strlen(s);
		if (i + 1 < len) total += sepn;
	}
	char *out = (char *)uli_arena_alloc(total + 1);
	char *w = out;
	for (int64_t i = 0; i < len; i++) {
		const char *s = data && data[i] ? data[i] : "";
		size_t n = strlen(s);
		memcpy(w, s, n);
		w += n;
		if (i + 1 < len && sepn) {
			memcpy(w, sep, sepn);
			w += sepn;
		}
	}
	*w = 0;
	return out;
}
