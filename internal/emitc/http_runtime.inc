/* Niraluli Tamil-0.59: HTTP/1.x server + client (Linux). Uses uli_net_* + GC park. */
#include <sys/time.h>

typedef struct {
	const char *method;
	const char *path;
	void *query;
	uli_slice_u8 body;
	void *headers;
} uli_http_req;

typedef struct {
	int64_t code;
	const char *status;
	void *headers;
	uli_slice_u8 body;
	void *err;
} uli_http_client_result;

typedef void (*uli_http_handler_fn)(void *env, uli_http_req req, int64_t w);

typedef struct {
	char *data;
	size_t len;
	size_t cap;
	void *err;
} uli_http_buf;

static const char *uli_http_status_text(int code) {
	switch (code) {
	case 200: return "OK";
	case 201: return "Created";
	case 202: return "Accepted";
	case 204: return "No Content";
	case 206: return "Partial Content";
	case 301: return "Moved Permanently";
	case 302: return "Found";
	case 303: return "See Other";
	case 304: return "Not Modified";
	case 307: return "Temporary Redirect";
	case 308: return "Permanent Redirect";
	case 400: return "Bad Request";
	case 401: return "Unauthorized";
	case 403: return "Forbidden";
	case 404: return "Not Found";
	case 405: return "Method Not Allowed";
	case 408: return "Request Timeout";
	case 409: return "Conflict";
	case 410: return "Gone";
	case 413: return "Content Too Large";
	case 415: return "Unsupported Media Type";
	case 422: return "Unprocessable Content";
	case 429: return "Too Many Requests";
	case 500: return "Internal Server Error";
	case 501: return "Not Implemented";
	case 502: return "Bad Gateway";
	case 503: return "Service Unavailable";
	case 504: return "Gateway Timeout";
	default: return "";
	}
}

static int uli_http_has_crlf(const char *s) {
	if (!s) return 0;
	for (const unsigned char *p = (const unsigned char *)s; *p; p++) {
		if (*p == '\r' || *p == '\n' || *p == '\0') return 1;
	}
	return 0;
}

static int uli_http_header_name_ok(const char *name) {
	if (!name || !*name) return 0;
	if (uli_http_has_crlf(name)) return 0;
	if (strchr(name, ':')) return 0;
	if (name[0] == ' ' || name[0] == '\t') return 0;
	size_t n = strlen(name);
	if (name[n - 1] == ' ' || name[n - 1] == '\t') return 0;
	return 1;
}

static int uli_http_header_reserved(const char *name) {
	return !strcasecmp(name, "Content-Length") ||
	       !strcasecmp(name, "Connection") ||
	       !strcasecmp(name, "Transfer-Encoding") ||
	       !strcasecmp(name, "Host");
}

static int uli_http_hex(unsigned char c) {
	if (c >= '0' && c <= '9') return c - '0';
	if (c >= 'a' && c <= 'f') return c - 'a' + 10;
	if (c >= 'A' && c <= 'F') return c - 'A' + 10;
	return -1;
}

static char *uli_http_query_decode(const char *s, void **err) {
	size_t n = strlen(s);
	char *out = (char *)uli_alloc(n + 1);
	size_t j = 0;
	for (size_t i = 0; i < n; i++) {
		unsigned char c = (unsigned char)s[i];
		if (c == '+') {
			out[j++] = ' ';
		} else if (c == '%') {
			if (i + 2 >= n) {
				*err = uli_net_error_new(EINVAL, "invalid query escape");
				return NULL;
			}
			int hi = uli_http_hex((unsigned char)s[i + 1]);
			int lo = uli_http_hex((unsigned char)s[i + 2]);
			if (hi < 0 || lo < 0 || (hi == 0 && lo == 0)) {
				*err = uli_net_error_new(EINVAL, "invalid query escape");
				return NULL;
			}
			out[j++] = (char)((hi << 4) | lo);
			i += 2;
		} else {
			out[j++] = (char)c;
		}
	}
	out[j] = '\0';
	return out;
}

static void *uli_http_parse_query(char *raw, void **err) {
	void *values = uli_http_headers_new();
	if (!raw || !*raw) return values;
	char *part = raw;
	for (;;) {
		char *amp = strchr(part, '&');
		if (amp) *amp = '\0';
		char *eq = strchr(part, '=');
		if (eq) *eq++ = '\0';
		char *key = uli_http_query_decode(part, err);
		if (*err) return NULL;
		char *value = uli_http_query_decode(eq ? eq : "", err);
		if (*err) return NULL;
		uli_http_headers_set(values, key, value);
		if (!amp) break;
		part = amp + 1;
	}
	return values;
}

static int64_t uli_http_header_i64(const char *headers, const char *key) {
	size_t klen = strlen(key);
	const char *p = headers;
	while (*p) {
		const char *eol = strstr(p, "\r\n");
		if (!eol) break;
		if ((size_t)(eol - p) > klen + 1 &&
		    strncasecmp(p, key, klen) == 0 && p[klen] == ':') {
			const char *v = p + klen + 1;
			while (*v == ' ' || *v == '\t') v++;
			return (int64_t)strtoll(v, NULL, 10);
		}
		if (eol[0] == '\0') break;
		p = eol + 2;
	}
	return -1;
}

static int uli_http_header_has(const char *headers, const char *key) {
	size_t klen = strlen(key);
	const char *p = headers;
	while (*p) {
		const char *eol = strstr(p, "\r\n");
		if (!eol) break;
		if ((size_t)(eol - p) > klen + 1 &&
		    strncasecmp(p, key, klen) == 0 && p[klen] == ':') {
			return 1;
		}
		if (eol[0] == '\0') break;
		p = eol + 2;
	}
	return 0;
}

static int uli_http_header_value_is(const char *headers, const char *key, const char *want) {
	size_t klen = strlen(key);
	const char *p = headers;
	while (*p) {
		const char *eol = strstr(p, "\r\n");
		if (!eol) break;
		if ((size_t)(eol - p) > klen + 1 &&
		    strncasecmp(p, key, klen) == 0 && p[klen] == ':') {
			const char *v = p + klen + 1;
			while (v < eol && (*v == ' ' || *v == '\t')) v++;
			const char *end = eol;
			while (end > v && (end[-1] == ' ' || end[-1] == '\t')) end--;
			return (size_t)(end - v) == strlen(want) &&
			       strncasecmp(v, want, (size_t)(end - v)) == 0;
		}
		p = eol + 2;
	}
	return 0;
}

static void uli_http_buf_init(uli_http_buf *b) {
	b->data = NULL;
	b->len = 0;
	b->cap = 0;
	b->err = NULL;
}

static void uli_http_buf_grow(uli_http_buf *b, size_t need) {
	if (b->err) return;
	if (b->len + need <= b->cap) return;
	size_t ncap = b->cap ? b->cap : 256;
	while (ncap < b->len + need) ncap *= 2;
	if (ncap > 65536) {
		b->err = uli_net_error_new(EOVERFLOW, "HTTP headers too large");
		return;
	}
	char *nb = (char *)uli_alloc(ncap);
	if (b->len && b->data) memcpy(nb, b->data, b->len);
	b->data = nb;
	b->cap = ncap;
}

static void uli_http_buf_append(uli_http_buf *b, const char *s, size_t n) {
	if (b->err) return;
	uli_http_buf_grow(b, n);
	if (b->err) return;
	memcpy(b->data + b->len, s, n);
	b->len += n;
}

static void uli_http_buf_append_str(uli_http_buf *b, const char *s) {
	if (!s) s = "";
	uli_http_buf_append(b, s, strlen(s));
}

static void uli_http_buf_appendf(uli_http_buf *b, const char *fmt, ...) {
	if (b->err) return;
	char tmp[512];
	va_list ap;
	va_start(ap, fmt);
	int n = vsnprintf(tmp, sizeof tmp, fmt, ap);
	va_end(ap);
	if (n < 0) {
		b->err = uli_net_error_new(EINVAL, "HTTP header formatting failed");
		return;
	}
	if ((size_t)n >= sizeof tmp) {
		b->err = uli_net_error_new(EOVERFLOW, "HTTP headers too large");
		return;
	}
	uli_http_buf_append(b, tmp, (size_t)n);
}

typedef struct {
	uli_http_buf *buf;
	void *err;
} uli_http_hdr_ctx;

static void uli_http_append_user_header(const char *k, const char *v, void *p) {
	uli_http_hdr_ctx *ctx = (uli_http_hdr_ctx *)p;
	if (ctx->err || ctx->buf->err) return;
	if (!uli_http_header_name_ok(k)) {
		ctx->err = uli_net_error_new(EINVAL, "invalid HTTP header name");
		return;
	}
	if (uli_http_has_crlf(v)) {
		ctx->err = uli_net_error_new(EINVAL, "invalid HTTP header value");
		return;
	}
	if (uli_http_header_reserved(k) || !strcasecmp(k, "Content-Type")) {
		ctx->err = uli_net_error_new(EINVAL, "reserved HTTP header");
		return;
	}
	uli_http_buf_append_str(ctx->buf, k);
	uli_http_buf_append_str(ctx->buf, ": ");
	uli_http_buf_append_str(ctx->buf, v ? v : "");
	uli_http_buf_append_str(ctx->buf, "\r\n");
}

static int uli_http_read_req(int64_t fd, uli_http_req *out, void **err) {
	size_t cap = 4096;
	char *buf = (char *)uli_alloc(cap);
	size_t n = 0;
	char *hdr_end = NULL;
	for (;;) {
		if (n + 512 >= cap) {
			if (cap >= 65536) {
				*err = uli_net_error_new(EOVERFLOW, "HTTP headers too large");
				return 0;
			}
			size_t ncap = cap * 2;
			char *nb = (char *)uli_alloc(ncap);
			memcpy(nb, buf, n);
			buf = nb;
			cap = ncap;
		}
		char sp;
		uli_gc_park_begin(&sp);
		ssize_t r = read((int)fd, buf + n, cap - n - 1);
		uli_gc_park_end();
		if (r < 0) {
			*err = uli_net_errno_error();
			return 0;
		}
		if (r == 0) break;
		n += (size_t)r;
		buf[n] = '\0';
		hdr_end = strstr(buf, "\r\n\r\n");
		if (hdr_end) break;
	}
	if (!hdr_end) {
		*err = uli_net_error_new(EINVAL, "malformed HTTP request");
		return 0;
	}
	size_t hdr_len = (size_t)(hdr_end - buf) + 4;
	char *line_end = strstr(buf, "\r\n");
	if (!line_end) {
		*err = uli_net_error_new(EINVAL, "malformed HTTP request");
		return 0;
	}
	*line_end = '\0';
	char *method = buf;
	char *path = strchr(buf, ' ');
	if (!path) {
		*err = uli_net_error_new(EINVAL, "malformed HTTP request");
		return 0;
	}
	*path++ = '\0';
	char *version = strchr(path, ' ');
	if (!version) {
		*err = uli_net_error_new(EINVAL, "malformed HTTP request");
		return 0;
	}
	*version++ = '\0';
	if ((strcmp(version, "HTTP/1.0") != 0 && strcmp(version, "HTTP/1.1") != 0) ||
	    !*method || path[0] != '/') {
		*err = uli_net_error_new(EINVAL, "unsupported HTTP request");
		return 0;
	}
	int64_t cl = uli_http_header_i64(line_end + 2, "Content-Length");
	void *headers = uli_http_headers_new();
	char *header = line_end + 2;
	while (header < hdr_end) {
		char *eol = strstr(header, "\r\n");
		if (!eol || eol > hdr_end) break;
		char *colon = memchr(header, ':', (size_t)(eol - header));
		if (colon) {
			*colon = '\0';
			*eol = '\0';
			for (char *p = header; *p; p++) {
				*p = (char)tolower((unsigned char)*p);
			}
			char *value = colon + 1;
			while (*value == ' ' || *value == '\t') value++;
			char *end = eol;
			while (end > value && (end[-1] == ' ' || end[-1] == '\t')) *--end = '\0';
			uli_http_headers_set(headers, header, value);
		}
		header = eol + 2;
	}
	size_t body_have = n > hdr_len ? n - hdr_len : 0;
	size_t body_need = cl > 0 ? (size_t)cl : 0;
	if (body_need > 1024 * 1024) {
		*err = uli_net_error_new(EOVERFLOW, "HTTP body too large");
		return 0;
	}
	uint8_t *body = NULL;
	if (body_need > 0) {
		body = (uint8_t *)uli_alloc(body_need);
		if (body_have > 0) {
			size_t copy = body_have < body_need ? body_have : body_need;
			memcpy(body, buf + hdr_len, copy);
			body_have = copy;
		} else {
			body_have = 0;
		}
		while (body_have < body_need) {
			char sp;
			uli_gc_park_begin(&sp);
			ssize_t r = read((int)fd, body + body_have, body_need - body_have);
			uli_gc_park_end();
			if (r <= 0) {
				*err = r < 0 ? uli_net_errno_error() : uli_net_error_new(EINVAL, "truncated HTTP body");
				return 0;
			}
			body_have += (size_t)r;
		}
	} else if (body_have > 0) {
		body = (uint8_t *)uli_alloc(body_have);
		memcpy(body, buf + hdr_len, body_have);
		body_need = body_have;
	}
	out->method = method;
	out->path = path;
	out->query = uli_http_headers_new();
	out->body = (uli_slice_u8){ body, (int64_t)body_need, (int64_t)body_need };
	out->headers = headers;
	char *query = strchr((char *)out->path, '?');
	if (query) {
		*query++ = '\0';
		out->query = uli_http_parse_query(query, err);
		if (*err) return 0;
	}
	return 1;
}

static void *uli_http_write(int64_t fd, int64_t code, const char *ctype, void *headers, const uint8_t *data, int64_t len) {
	if (len < 0) return uli_net_error_new(EINVAL, "invalid HTTP body");
	if (code < 100 || code > 999) return uli_net_error_new(EINVAL, "invalid HTTP status code");
	if (!ctype) ctype = "application/octet-stream";
	if (uli_http_has_crlf(ctype)) return uli_net_error_new(EINVAL, "invalid Content-Type");
	const char *text = uli_http_status_text((int)code);
	uli_http_buf buf;
	uli_http_buf_init(&buf);
	uli_http_buf_appendf(&buf, "HTTP/1.0 %d %s\r\nContent-Type: %s\r\n", (int)code, text, ctype);
	if (buf.err) return buf.err;
	if (headers) {
		uli_http_hdr_ctx ctx = { &buf, NULL };
		uli_http_headers_each(headers, uli_http_append_user_header, &ctx);
		if (ctx.err) return ctx.err;
		if (buf.err) return buf.err;
	}
	uli_http_buf_appendf(&buf, "Content-Length: %lld\r\nConnection: close\r\n\r\n", (long long)len);
	if (buf.err) return buf.err;
	size_t total = buf.len + (size_t)len;
	uint8_t *msg = (uint8_t *)uli_alloc(total);
	memcpy(msg, buf.data, buf.len);
	if (len > 0) {
		if (!data) return uli_net_error_new(EINVAL, "invalid HTTP body");
		memcpy(msg + buf.len, data, (size_t)len);
	}
	uli_net_i64_result written = uli_net_write(fd, msg, (int64_t)total);
	return written.err;
}

static void *uli_http_write_str(int64_t fd, int64_t code, const char *ctype, void *headers, const char *s) {
	size_t n = s ? strlen(s) : 0;
	return uli_http_write(fd, code, ctype, headers, (const uint8_t *)s, (int64_t)n);
}

static void *uli_http_handle_conn(int64_t fd, uli_fn handler) {
	uli_http_req req;
	memset(&req, 0, sizeof req);
	void *err = NULL;
	if (!uli_http_read_req(fd, &req, &err)) {
		uli_http_write_str(fd, 400, "text/plain; charset=utf-8", NULL, "bad request");
		return err;
	}
	((uli_http_handler_fn)handler.fn)(handler.env, req, fd);
	return NULL;
}

static void *uli_http_serve_one(int64_t ln, uli_fn handler) {
	uli_net_i64_result accepted = uli_net_accept(ln);
	if (accepted.err) return accepted.err;
	void *err = uli_http_handle_conn(accepted.value, handler);
	void *close_err = uli_net_close(accepted.value);
	return err ? err : close_err;
}

typedef struct {
	uli_fn handler;
	int64_t fd;
} uli_http_job;

static void *uli_http_job_main(void *p) {
	uli_thread_register();
	uli_http_job *j = (uli_http_job *)p;
	uli_http_handle_conn(j->fd, j->handler);
	uli_net_close(j->fd);
	free(j);
	uli_thread_unregister();
	return NULL;
}

static void *uli_http_listen_and_serve(const char *addr, uli_fn handler) {
	uli_net_i64_result listened = uli_net_listen(addr);
	if (listened.err) return listened.err;
	int64_t ln = listened.value;
	for (;;) {
		uli_gc_poll();
		uli_net_i64_result accepted = uli_net_accept(ln);
		if (accepted.err) return accepted.err;
		uli_http_job *j = (uli_http_job *)malloc(sizeof(*j));
		if (!j) return uli_net_error_new(ENOMEM, "HTTP worker allocation failed");
		j->handler = handler;
		j->fd = accepted.value;
		pthread_t th;
		if (pthread_create(&th, NULL, uli_http_job_main, j) != 0) {
			free(j);
			return uli_net_error_new(errno, strerror(errno));
		}
		pthread_detach(th);
	}
}

/* ---- Tamil-0.59 HTTP client ---- */

typedef struct {
	char host[256];
	char port[16];
	char path[2048];
	char authority[280];
} uli_http_url;

static uli_http_client_result uli_http_client_fail(void *err) {
	return (uli_http_client_result){ 0, "", NULL, (uli_slice_u8){0}, err };
}

static void *uli_http_parse_url(const char *url, uli_http_url *out) {
	memset(out, 0, sizeof *out);
	if (!url || !*url) return uli_net_error_new(EINVAL, "invalid HTTP URL");
	if (!strncmp(url, "https://", 8)) {
#ifndef EPROTONOSUPPORT
#define EPROTONOSUPPORT ENOPROTOOPT
#endif
		return uli_net_error_new(EPROTONOSUPPORT, "HTTPS not supported");
	}
	if (strncmp(url, "http://", 7) != 0) return uli_net_error_new(EINVAL, "invalid HTTP URL");
	const char *rest = url + 7;
	if (!*rest) return uli_net_error_new(EINVAL, "invalid HTTP URL");
	if (strchr(rest, '@')) return uli_net_error_new(EINVAL, "HTTP userinfo not supported");
	if (rest[0] == '[') return uli_net_error_new(EINVAL, "IPv6 literals not supported");
	const char *slash = strchr(rest, '/');
	const char *q = strchr(rest, '?');
	const char *hash = strchr(rest, '#');
	const char *auth_end = rest + strlen(rest);
	if (slash && slash < auth_end) auth_end = slash;
	if (q && q < auth_end) auth_end = q;
	if (hash && hash < auth_end) auth_end = hash;
	size_t auth_len = (size_t)(auth_end - rest);
	if (auth_len == 0 || auth_len >= sizeof out->authority) return uli_net_error_new(EINVAL, "invalid HTTP URL");
	memcpy(out->authority, rest, auth_len);
	out->authority[auth_len] = '\0';
	const char *colon = strrchr(out->authority, ':');
	if (colon) {
		size_t hlen = (size_t)(colon - out->authority);
		if (hlen == 0 || hlen >= sizeof out->host) return uli_net_error_new(EINVAL, "invalid HTTP URL");
		memcpy(out->host, out->authority, hlen);
		out->host[hlen] = '\0';
		if (!colon[1]) return uli_net_error_new(EINVAL, "invalid HTTP URL");
		for (const char *p = colon + 1; *p; p++) {
			if (*p < '0' || *p > '9') return uli_net_error_new(EINVAL, "invalid HTTP URL");
		}
		if (strlen(colon + 1) >= sizeof out->port) return uli_net_error_new(EINVAL, "invalid HTTP URL");
		strcpy(out->port, colon + 1);
	} else {
		if (strlen(out->authority) >= sizeof out->host) return uli_net_error_new(EINVAL, "invalid HTTP URL");
		strcpy(out->host, out->authority);
		strcpy(out->port, "80");
	}
	const char *path_start = auth_end;
	const char *path_end = path_start + strlen(path_start);
	if (hash && hash >= path_start && hash < path_end) path_end = hash;
	size_t plen = (size_t)(path_end - path_start);
	if (plen == 0) {
		strcpy(out->path, "/");
	} else {
		if (plen >= sizeof out->path) return uli_net_error_new(EINVAL, "invalid HTTP URL");
		memcpy(out->path, path_start, plen);
		out->path[plen] = '\0';
	}
	return NULL;
}

static void uli_http_parse_headers_block(char *block, char *hdr_end, void *headers) {
	char *header = block;
	while (header < hdr_end) {
		char *eol = strstr(header, "\r\n");
		if (!eol || eol > hdr_end) break;
		char *colon = memchr(header, ':', (size_t)(eol - header));
		if (colon) {
			*colon = '\0';
			*eol = '\0';
			for (char *p = header; *p; p++) *p = (char)tolower((unsigned char)*p);
			char *value = colon + 1;
			while (*value == ' ' || *value == '\t') value++;
			char *end = eol;
			while (end > value && (end[-1] == ' ' || end[-1] == '\t')) *--end = '\0';
			uli_http_headers_set(headers, header, value);
		}
		header = eol + 2;
	}
}

static void *uli_http_decode_chunked(const uint8_t *src, size_t n, uint8_t **body_out, size_t *len_out) {
	const size_t limit = 1024 * 1024;
	size_t cap = 256;
	uint8_t *body = (uint8_t *)uli_alloc(cap);
	size_t pos = 0;
	size_t used = 0;
	for (;;) {
		size_t line = pos;
		while (line + 1 < n && !(src[line] == '\r' && src[line + 1] == '\n')) line++;
		if (line + 1 >= n) return uli_net_error_new(EINVAL, "truncated chunked HTTP body");
		size_t end = pos;
		while (end < line && src[end] != ';') end++;
		if (end == pos) return uli_net_error_new(EINVAL, "invalid HTTP chunk size");
		size_t chunk = 0;
		for (size_t i = pos; i < end; i++) {
			int digit = uli_http_hex(src[i]);
			if (digit < 0 || chunk > (limit >> 4)) {
				return uli_net_error_new(EINVAL, "invalid HTTP chunk size");
			}
			chunk = (chunk << 4) | (size_t)digit;
		}
		pos = line + 2;
		if (chunk == 0) {
			for (;;) {
				size_t trailer = pos;
				while (trailer + 1 < n && !(src[trailer] == '\r' && src[trailer + 1] == '\n')) trailer++;
				if (trailer + 1 >= n) return uli_net_error_new(EINVAL, "truncated HTTP trailers");
				if (trailer == pos) {
					*body_out = body;
					*len_out = used;
					return NULL;
				}
				pos = trailer + 2;
			}
		}
		if (chunk > limit - used) return uli_net_error_new(EOVERFLOW, "HTTP body too large");
		if (used + chunk > cap) {
			size_t next = cap;
			while (next < used + chunk) next *= 2;
			if (next > limit) next = limit;
			uint8_t *grown = (uint8_t *)uli_alloc(next);
			if (used) memcpy(grown, body, used);
			body = grown;
			cap = next;
		}
		if (chunk > n - pos || n - pos - chunk < 2 ||
		    src[pos + chunk] != '\r' || src[pos + chunk + 1] != '\n') {
			return uli_net_error_new(EINVAL, "truncated chunked HTTP body");
		}
		memcpy(body + used, src + pos, chunk);
		used += chunk;
		pos += chunk + 2;
	}
}

static uli_http_client_result uli_http_read_response(int64_t fd) {
	size_t cap = 4096;
	char *buf = (char *)uli_alloc(cap);
	size_t n = 0;
	char *hdr_end = NULL;
	for (;;) {
		if (n + 512 >= cap) {
			if (cap >= 65536) {
				uli_net_close(fd);
				return uli_http_client_fail(uli_net_error_new(EOVERFLOW, "HTTP headers too large"));
			}
			size_t ncap = cap * 2;
			char *nb = (char *)uli_alloc(ncap);
			memcpy(nb, buf, n);
			buf = nb;
			cap = ncap;
		}
		char sp;
		uli_gc_park_begin(&sp);
		ssize_t r = read((int)fd, buf + n, cap - n - 1);
		uli_gc_park_end();
		if (r < 0) {
			void *e = uli_net_errno_error();
			uli_net_close(fd);
			return uli_http_client_fail(e);
		}
		if (r == 0) break;
		n += (size_t)r;
		buf[n] = '\0';
		hdr_end = strstr(buf, "\r\n\r\n");
		if (hdr_end) break;
	}
	if (!hdr_end) {
		uli_net_close(fd);
		return uli_http_client_fail(uli_net_error_new(EINVAL, "malformed HTTP response"));
	}
	size_t hdr_len = (size_t)(hdr_end - buf) + 4;
	char *line_end = strstr(buf, "\r\n");
	if (!line_end) {
		uli_net_close(fd);
		return uli_http_client_fail(uli_net_error_new(EINVAL, "malformed HTTP response"));
	}
	*line_end = '\0';
	char *proto = buf;
	char *code_s = strchr(proto, ' ');
	if (!code_s) {
		uli_net_close(fd);
		return uli_http_client_fail(uli_net_error_new(EINVAL, "malformed HTTP response"));
	}
	*code_s++ = '\0';
	if (strcmp(proto, "HTTP/1.0") != 0 && strcmp(proto, "HTTP/1.1") != 0) {
		uli_net_close(fd);
		return uli_http_client_fail(uli_net_error_new(EINVAL, "unsupported HTTP version"));
	}
	char *reason = strchr(code_s, ' ');
	if (reason) *reason++ = '\0';
	else reason = "";
	if (strlen(code_s) != 3 || code_s[0] < '1' || code_s[0] > '9' ||
	    code_s[1] < '0' || code_s[1] > '9' || code_s[2] < '0' || code_s[2] > '9') {
		uli_net_close(fd);
		return uli_http_client_fail(uli_net_error_new(EINVAL, "malformed HTTP response"));
	}
	int64_t code = (int64_t)strtoll(code_s, NULL, 10);
	char *status = (char *)uli_alloc(strlen(code_s) + 1 + strlen(reason) + 1);
	if (*reason) sprintf(status, "%s %s", code_s, reason);
	else strcpy(status, code_s);
	int has_transfer = uli_http_header_has(line_end + 2, "Transfer-Encoding");
	int chunked = uli_http_header_value_is(line_end + 2, "Transfer-Encoding", "chunked");
	if (has_transfer && !chunked) {
		uli_net_close(fd);
		return uli_http_client_fail(uli_net_error_new(EINVAL, "unsupported HTTP transfer encoding"));
	}
	void *resp_headers = uli_http_headers_new();
	uli_http_parse_headers_block(line_end + 2, hdr_end, resp_headers);
	int64_t cl = uli_http_header_i64(line_end + 2, "Content-Length");
	size_t body_have = n > hdr_len ? n - hdr_len : 0;
	uint8_t *body = NULL;
	size_t body_need = 0;
	if (!chunked && cl >= 0) {
		body_need = (size_t)cl;
		if (body_need > 1024 * 1024) {
			uli_net_close(fd);
			return uli_http_client_fail(uli_net_error_new(EOVERFLOW, "HTTP body too large"));
		}
		body = (uint8_t *)uli_alloc(body_need ? body_need : 1);
		if (body_have > 0) {
			size_t copy = body_have < body_need ? body_have : body_need;
			memcpy(body, buf + hdr_len, copy);
			body_have = copy;
		} else body_have = 0;
		while (body_have < body_need) {
			char sp;
			uli_gc_park_begin(&sp);
			ssize_t r = read((int)fd, body + body_have, body_need - body_have);
			uli_gc_park_end();
			if (r <= 0) {
				void *e = r < 0 ? uli_net_errno_error() : uli_net_error_new(EINVAL, "truncated HTTP body");
				uli_net_close(fd);
				return uli_http_client_fail(e);
			}
			body_have += (size_t)r;
		}
	} else {
		size_t body_limit = chunked ? 2 * 1024 * 1024 : 1024 * 1024;
		size_t bcap = body_have > 256 ? body_have : 256;
		body = (uint8_t *)uli_alloc(bcap);
		if (body_have) memcpy(body, buf + hdr_len, body_have);
		body_need = body_have;
		for (;;) {
			if (body_need + 512 > bcap) {
				if (bcap >= body_limit) {
					uli_net_close(fd);
					return uli_http_client_fail(uli_net_error_new(EOVERFLOW, "HTTP body too large"));
				}
				size_t ncap = bcap * 2;
				if (ncap > body_limit) ncap = body_limit;
				uint8_t *nb = (uint8_t *)uli_alloc(ncap);
				if (body_need) memcpy(nb, body, body_need);
				body = nb;
				bcap = ncap;
			}
			char sp;
			uli_gc_park_begin(&sp);
			ssize_t r = read((int)fd, body + body_need, bcap - body_need);
			uli_gc_park_end();
			if (r < 0) {
				void *e = uli_net_errno_error();
				uli_net_close(fd);
				return uli_http_client_fail(e);
			}
			if (r == 0) break;
			body_need += (size_t)r;
		}
	}
	if (chunked) {
		uint8_t *decoded = NULL;
		size_t decoded_len = 0;
		void *chunk_err = uli_http_decode_chunked(body, body_need, &decoded, &decoded_len);
		if (chunk_err) {
			uli_net_close(fd);
			return uli_http_client_fail(chunk_err);
		}
		body = decoded;
		body_need = decoded_len;
	}
	uli_net_close(fd);
	return (uli_http_client_result){
		code,
		status,
		resp_headers,
		(uli_slice_u8){ body, (int64_t)body_need, (int64_t)body_need },
		NULL
	};
}

static int uli_http_redirect_code(int64_t code) {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308;
}

/* Resolve a redirect Location against base URL. Absolute http(s):// kept;
   absolute paths (/…) and relative paths join the base origin/path. */
static const char *uli_http_resolve_location(const char *base, const char *loc) {
	if (!loc || !*loc) return NULL;
	if (!strncmp(loc, "http://", 7) || !strncmp(loc, "https://", 8)) return loc;
	uli_http_url base_u;
	if (uli_http_parse_url(base, &base_u)) return NULL;
	char pathbuf[2048];
	if (loc[0] == '/') {
		if (strlen(loc) >= sizeof pathbuf) return NULL;
		strcpy(pathbuf, loc);
	} else {
		const char *bp = base_u.path[0] ? base_u.path : "/";
		const char *slash = strrchr(bp, '/');
		size_t dir_len = slash ? (size_t)(slash - bp) + 1 : 1;
		if (dir_len == 0) dir_len = 1;
		if (dir_len + strlen(loc) >= sizeof pathbuf) return NULL;
		if (!slash) {
			pathbuf[0] = '/';
			pathbuf[1] = '\0';
			dir_len = 1;
		} else {
			memcpy(pathbuf, bp, dir_len);
			pathbuf[dir_len] = '\0';
		}
		strcpy(pathbuf + dir_len, loc);
	}
	char *out = (char *)uli_alloc(8 + strlen(base_u.authority) + strlen(pathbuf) + 1);
	sprintf(out, "http://%s%s", base_u.authority, pathbuf);
	return out;
}

static void *uli_http_set_timeout(int64_t fd, int64_t timeout_ms) {
	if (timeout_ms < 0) return uli_net_error_new(EINVAL, "invalid HTTP timeout");
	if (timeout_ms == 0) return NULL;
	struct timeval tv = {
		.tv_sec = (time_t)(timeout_ms / 1000),
		.tv_usec = (suseconds_t)((timeout_ms % 1000) * 1000),
	};
	if (setsockopt((int)fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof tv) != 0 ||
	    setsockopt((int)fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof tv) != 0)
		return uli_net_errno_error();
	return NULL;
}

static uli_http_client_result uli_http_do_ex(const char *method, const char *url, void *headers, const char *ctype, const uint8_t *data, int64_t len, int64_t timeout_ms) {
	if (!method || !*method) method = "GET";
	if (uli_http_has_crlf(method) || strchr(method, ' ')) {
		return uli_http_client_fail(uli_net_error_new(EINVAL, "invalid HTTP method"));
	}
	if (len < 0) return uli_http_client_fail(uli_net_error_new(EINVAL, "invalid HTTP body"));
	if (ctype && uli_http_has_crlf(ctype)) {
		return uli_http_client_fail(uli_net_error_new(EINVAL, "invalid Content-Type"));
	}
	uli_http_url parsed;
	void *err = uli_http_parse_url(url, &parsed);
	if (err) return uli_http_client_fail(err);

	char dial[300];
	int dn = snprintf(dial, sizeof dial, "%s:%s", parsed.host, parsed.port);
	if (dn < 0 || (size_t)dn >= sizeof dial) {
		return uli_http_client_fail(uli_net_error_new(EINVAL, "invalid HTTP URL"));
	}
	uli_net_i64_result dialed = timeout_ms > 0 ? uli_net_dial_timeout(dial, timeout_ms) : uli_net_dial(dial);
	if (dialed.err) return uli_http_client_fail(dialed.err);
	int64_t fd = dialed.value;
	err = uli_http_set_timeout(fd, timeout_ms);
	if (err) {
		uli_net_close(fd);
		return uli_http_client_fail(err);
	}

	uli_http_buf req;
	uli_http_buf_init(&req);
	uli_http_buf_appendf(&req, "%s %s HTTP/1.0\r\nHost: %s\r\n", method, parsed.path, parsed.authority);
	if (req.err) {
		uli_net_close(fd);
		return uli_http_client_fail(req.err);
	}
	if (ctype && *ctype) {
		uli_http_buf_appendf(&req, "Content-Type: %s\r\n", ctype);
		if (req.err) {
			uli_net_close(fd);
			return uli_http_client_fail(req.err);
		}
	}
	if (headers) {
		uli_http_hdr_ctx ctx = { &req, NULL };
		uli_http_headers_each(headers, uli_http_append_user_header, &ctx);
		if (ctx.err || req.err) {
			uli_net_close(fd);
			return uli_http_client_fail(ctx.err ? ctx.err : req.err);
		}
	}
	uli_http_buf_appendf(&req, "Content-Length: %lld\r\nConnection: close\r\n\r\n", (long long)(len < 0 ? 0 : len));
	if (req.err) {
		uli_net_close(fd);
		return uli_http_client_fail(req.err);
	}
	size_t total = req.len + (size_t)(len > 0 ? len : 0);
	uint8_t *msg = (uint8_t *)uli_alloc(total);
	memcpy(msg, req.data, req.len);
	if (len > 0) {
		if (!data) {
			uli_net_close(fd);
			return uli_http_client_fail(uli_net_error_new(EINVAL, "invalid HTTP body"));
		}
		memcpy(msg + req.len, data, (size_t)len);
	}
	uli_net_i64_result written = uli_net_write(fd, msg, (int64_t)total);
	if (written.err) {
		uli_net_close(fd);
		return uli_http_client_fail(written.err);
	}
	return uli_http_read_response(fd);
}

static uli_http_client_result uli_http_do(const char *method, const char *url, void *headers, const uint8_t *data, int64_t len) {
	return uli_http_do_ex(method, url, headers, NULL, data, len, 0);
}

static uli_http_client_result uli_http_get(const char *url) {
	return uli_http_do_ex("GET", url, NULL, NULL, NULL, 0, 0);
}

static uli_http_client_result uli_http_post(const char *url, const char *ctype, const uint8_t *data, int64_t len) {
	return uli_http_do_ex("POST", url, NULL, ctype && *ctype ? ctype : "application/octet-stream", data, len, 0);
}

static uli_http_client_result uli_http_do_opts(const char *method, const char *url, void *headers, const uint8_t *data, int64_t len, int64_t timeout_ms, int64_t max_redirects) {
	if (max_redirects < 0) return uli_http_client_fail(uli_net_error_new(EINVAL, "invalid HTTP redirect limit"));
	const char *current_url = url;
	const char *current_method = method;
	const uint8_t *current_data = data;
	int64_t current_len = len;
	for (int64_t redirects = 0;; redirects++) {
		uli_http_client_result result = uli_http_do_ex(current_method, current_url, headers, NULL, current_data, current_len, timeout_ms);
		if (result.err || !uli_http_redirect_code(result.code) || max_redirects == 0) return result;
		if (redirects >= max_redirects) return uli_http_client_fail(uli_net_error_new(ELOOP, "HTTP redirect limit exceeded"));
		const char *location = uli_http_headers_get(result.headers, "location");
		if (!location || !*location)
			return uli_http_client_fail(uli_net_error_new(EINVAL, "unsupported HTTP redirect location"));
		const char *resolved = uli_http_resolve_location(current_url, location);
		if (!resolved || strncmp(resolved, "http://", 7) != 0)
			return uli_http_client_fail(uli_net_error_new(EINVAL, "unsupported HTTP redirect location"));
		current_url = resolved;
		if (result.code == 303 || ((result.code == 301 || result.code == 302) && strcasecmp(current_method, "GET") && strcasecmp(current_method, "HEAD"))) {
			current_method = "GET";
			current_data = NULL;
			current_len = 0;
		}
	}
}
