/* Niraluli Tamil-0.55/0.80: TCP sockets (Linux). Blocking I/O parks for GC. */
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <poll.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <unistd.h>

typedef struct { int64_t value; void *err; } uli_net_i64_result;
typedef struct { const char *value; void *err; } uli_net_str_result;
typedef struct { int64_t n; const char *addr; void *err; } uli_net_udp_result;
typedef struct { int64_t code; const char *message; } uli_net_error;

static void *uli_net_error_new(int64_t code, const char *message) {
	uli_net_error *err = (uli_net_error *)uli_alloc(sizeof(*err));
	err->code = code;
	err->message = message ? message : "network error";
	return err;
}

static void *uli_net_errno_error(void) {
	return uli_net_error_new(errno, strerror(errno));
}

static int uli_net_hostport(const char *addr, char *host, size_t hostcap, const char **port) {
	if (!addr || !host || !port || hostcap < 2) return -1;
	const char *colon = strrchr(addr, ':');
	if (!colon || colon[1] == '\0') return -1;
	*port = colon + 1;
	size_t hl = (size_t)(colon - addr);
	if (hl >= hostcap) return -1;
	if (hl == 0) {
		host[0] = '\0';
		return 0;
	}
	memcpy(host, addr, hl);
	host[hl] = '\0';
	return 0;
}

static uli_net_i64_result uli_net_listen(const char *addr) {
	char host[256];
	const char *port = NULL;
	if (uli_net_hostport(addr, host, sizeof host, &port) != 0)
		return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid listen address") };
	struct addrinfo hints;
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_STREAM;
	hints.ai_flags = AI_PASSIVE | AI_NUMERICSERV;
	const char *node = host[0] ? host : NULL;
	struct addrinfo *res = NULL;
	int gai = getaddrinfo(node, port, &hints, &res);
	if (gai != 0 || !res)
		return (uli_net_i64_result){ 0, uli_net_error_new(gai, gai_strerror(gai)) };
	int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
	if (fd < 0) {
		freeaddrinfo(res);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	int yes = 1;
	setsockopt(fd, SOL_SOCKET, SO_REUSEADDR, &yes, sizeof yes);
	if (bind(fd, res->ai_addr, res->ai_addrlen) != 0) {
		close(fd);
		freeaddrinfo(res);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	if (listen(fd, 16) != 0) {
		close(fd);
		freeaddrinfo(res);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	freeaddrinfo(res);
	return (uli_net_i64_result){ fd, NULL };
}

static uli_net_i64_result uli_net_accept(int64_t ln) {
	char sp;
	uli_gc_park_begin(&sp);
	int fd = accept((int)ln, NULL, NULL);
	uli_gc_park_end();
	if (fd < 0) return (uli_net_i64_result){ 0, uli_net_errno_error() };
	return (uli_net_i64_result){ fd, NULL };
}

static uli_net_i64_result uli_net_dial(const char *addr) {
	char host[256];
	const char *port = NULL;
	if (uli_net_hostport(addr, host, sizeof host, &port) != 0 || !host[0])
		return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid dial address") };
	struct addrinfo hints;
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_STREAM;
	hints.ai_flags = AI_NUMERICSERV;
	struct addrinfo *res = NULL;
	int gai = getaddrinfo(host, port, &hints, &res);
	if (gai != 0 || !res)
		return (uli_net_i64_result){ 0, uli_net_error_new(gai, gai_strerror(gai)) };
	int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
	if (fd < 0) {
		freeaddrinfo(res);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	char sp;
	uli_gc_park_begin(&sp);
	int rc = connect(fd, res->ai_addr, res->ai_addrlen);
	uli_gc_park_end();
	freeaddrinfo(res);
	if (rc != 0) {
		close(fd);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	return (uli_net_i64_result){ fd, NULL };
}

/* Tamil-0.80: dial with connect timeout (ms). DNS resolve still parks but is not
   separately capped; timeout applies to the non-blocking connect/poll phase. */
static uli_net_i64_result uli_net_dial_timeout(const char *addr, int64_t timeout_ms) {
	if (timeout_ms <= 0) return uli_net_dial(addr);
	char host[256];
	const char *port = NULL;
	if (uli_net_hostport(addr, host, sizeof host, &port) != 0 || !host[0])
		return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid dial address") };
	struct addrinfo hints;
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_STREAM;
	hints.ai_flags = AI_NUMERICSERV;
	struct addrinfo *res = NULL;
	char sp;
	uli_gc_park_begin(&sp);
	int gai = getaddrinfo(host, port, &hints, &res);
	uli_gc_park_end();
	if (gai != 0 || !res)
		return (uli_net_i64_result){ 0, uli_net_error_new(gai, gai_strerror(gai)) };
	int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
	if (fd < 0) {
		freeaddrinfo(res);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	int flags = fcntl(fd, F_GETFL, 0);
	if (flags < 0 || fcntl(fd, F_SETFL, flags | O_NONBLOCK) != 0) {
		close(fd);
		freeaddrinfo(res);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	uli_gc_park_begin(&sp);
	int rc = connect(fd, res->ai_addr, res->ai_addrlen);
	int cerr = errno;
	uli_gc_park_end();
	freeaddrinfo(res);
	if (rc == 0) {
		fcntl(fd, F_SETFL, flags);
		return (uli_net_i64_result){ fd, NULL };
	}
	if (cerr != EINPROGRESS) {
		close(fd);
		return (uli_net_i64_result){ 0, uli_net_error_new(cerr, strerror(cerr)) };
	}
	struct pollfd pfd = { .fd = fd, .events = POLLOUT };
	uli_gc_park_begin(&sp);
	int pr = poll(&pfd, 1, (int)timeout_ms);
	uli_gc_park_end();
	if (pr == 0) {
		close(fd);
		return (uli_net_i64_result){ 0, uli_net_error_new(ETIMEDOUT, "dial timeout") };
	}
	if (pr < 0) {
		int e = errno;
		close(fd);
		return (uli_net_i64_result){ 0, uli_net_error_new(e, strerror(e)) };
	}
	int soerr = 0;
	socklen_t slen = sizeof soerr;
	if (getsockopt(fd, SOL_SOCKET, SO_ERROR, &soerr, &slen) != 0) {
		close(fd);
		return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	if (soerr != 0) {
		close(fd);
		return (uli_net_i64_result){ 0, uli_net_error_new(soerr, strerror(soerr)) };
	}
	fcntl(fd, F_SETFL, flags);
	return (uli_net_i64_result){ fd, NULL };
}

static uli_net_i64_result uli_net_read(int64_t fd, uint8_t *data, int64_t len) {
	if (len < 0) return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid read buffer") };
	if (len == 0) return (uli_net_i64_result){ 0, NULL };
	if (!data) return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid read buffer") };
	char sp;
	uli_gc_park_begin(&sp);
	ssize_t n = read((int)fd, data, (size_t)len);
	uli_gc_park_end();
	if (n < 0) return (uli_net_i64_result){ 0, uli_net_errno_error() };
	return (uli_net_i64_result){ (int64_t)n, NULL };
}

static uli_net_i64_result uli_net_write(int64_t fd, const uint8_t *data, int64_t len) {
	if (len < 0) return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid write buffer") };
	if (len == 0) return (uli_net_i64_result){ 0, NULL };
	if (!data) return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid write buffer") };
	int64_t off = 0;
	while (off < len) {
		char sp;
		uli_gc_park_begin(&sp);
		ssize_t n = write((int)fd, data + off, (size_t)(len - off));
		uli_gc_park_end();
		if (n <= 0) return (uli_net_i64_result){ off, uli_net_errno_error() };
		off += n;
	}
	return (uli_net_i64_result){ off, NULL };
}

static void *uli_net_close(int64_t fd) {
	if (fd >= 0 && close((int)fd) != 0) return uli_net_errno_error();
	return NULL;
}

static uli_net_str_result uli_net_addr(int64_t fd) {
	struct sockaddr_in sa;
	socklen_t slen = sizeof sa;
	memset(&sa, 0, sizeof sa);
	if (getsockname((int)fd, (struct sockaddr *)&sa, &slen) != 0)
		return (uli_net_str_result){ "", uli_net_errno_error() };
	char ip[INET_ADDRSTRLEN];
	if (!inet_ntop(AF_INET, &sa.sin_addr, ip, sizeof ip))
		return (uli_net_str_result){ "", uli_net_errno_error() };
	if (strcmp(ip, "0.0.0.0") == 0) memcpy(ip, "127.0.0.1", 10);
	unsigned port = ntohs(sa.sin_port);
	char buf[64];
	int n = snprintf(buf, sizeof buf, "%s:%u", ip, port);
	if (n < 0 || (size_t)n >= sizeof buf)
		return (uli_net_str_result){ "", uli_net_error_new(EINVAL, "address formatting failed") };
	char *r = (char *)uli_alloc((size_t)n + 1);
	memcpy(r, buf, (size_t)n + 1);
	return (uli_net_str_result){ r, NULL };
}

static uli_net_i64_result uli_net_udp_listen(const char *addr) {
	char host[256];
	const char *port = NULL;
	if (uli_net_hostport(addr, host, sizeof host, &port) != 0)
		return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid UDP listen address") };
	struct addrinfo hints;
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_DGRAM;
	hints.ai_flags = AI_PASSIVE | AI_NUMERICSERV;
	struct addrinfo *res = NULL;
	int gai = getaddrinfo(host[0] ? host : NULL, port, &hints, &res);
	if (gai != 0 || !res) return (uli_net_i64_result){ 0, uli_net_error_new(gai, gai_strerror(gai)) };
	int fd = socket(res->ai_family, res->ai_socktype, res->ai_protocol);
	if (fd < 0) { freeaddrinfo(res); return (uli_net_i64_result){ 0, uli_net_errno_error() }; }
	if (bind(fd, res->ai_addr, res->ai_addrlen) != 0) {
		close(fd); freeaddrinfo(res); return (uli_net_i64_result){ 0, uli_net_errno_error() };
	}
	freeaddrinfo(res);
	return (uli_net_i64_result){ fd, NULL };
}

static uli_net_i64_result uli_net_udp_send(int64_t fd, const char *addr, const uint8_t *data, int64_t len) {
	char host[256];
	const char *port = NULL;
	if (len < 0 || (len > 0 && !data)) return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid UDP data") };
	if (uli_net_hostport(addr, host, sizeof host, &port) != 0 || !host[0])
		return (uli_net_i64_result){ 0, uli_net_error_new(EINVAL, "invalid UDP address") };
	struct addrinfo hints;
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_INET;
	hints.ai_socktype = SOCK_DGRAM;
	hints.ai_flags = AI_NUMERICSERV;
	struct addrinfo *res = NULL;
	int gai = getaddrinfo(host, port, &hints, &res);
	if (gai != 0 || !res) return (uli_net_i64_result){ 0, uli_net_error_new(gai, gai_strerror(gai)) };
	volatile const uint8_t *root = data;
	char sp;
	uli_gc_park_begin(&sp);
	ssize_t n = sendto((int)fd, (const uint8_t *)root, (size_t)len, 0, res->ai_addr, res->ai_addrlen);
	uli_gc_park_end();
	freeaddrinfo(res);
	if (n < 0) return (uli_net_i64_result){ 0, uli_net_errno_error() };
	return (uli_net_i64_result){ (int64_t)n, NULL };
}

static uli_net_udp_result uli_net_udp_recv(int64_t fd, uint8_t *data, int64_t len) {
	if (len < 0 || (len > 0 && !data)) return (uli_net_udp_result){ 0, "", uli_net_error_new(EINVAL, "invalid UDP buffer") };
	struct sockaddr_in peer;
	socklen_t plen = sizeof peer;
	volatile uint8_t *root = data;
	char sp;
	uli_gc_park_begin(&sp);
	ssize_t n = recvfrom((int)fd, (uint8_t *)root, (size_t)len, 0, (struct sockaddr *)&peer, &plen);
	uli_gc_park_end();
	if (n < 0) return (uli_net_udp_result){ 0, "", uli_net_errno_error() };
	char ip[INET_ADDRSTRLEN];
	if (!inet_ntop(AF_INET, &peer.sin_addr, ip, sizeof ip)) return (uli_net_udp_result){ 0, "", uli_net_errno_error() };
	char buf[64];
	int m = snprintf(buf, sizeof buf, "%s:%u", ip, ntohs(peer.sin_port));
	if (m < 0 || (size_t)m >= sizeof buf) return (uli_net_udp_result){ 0, "", uli_net_error_new(EINVAL, "address formatting failed") };
	char *out = (char *)uli_alloc((size_t)m + 1);
	memcpy(out, buf, (size_t)m + 1);
	return (uli_net_udp_result){ (int64_t)n, out, NULL };
}
