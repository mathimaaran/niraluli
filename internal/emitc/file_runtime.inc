/* Niraluli Tamil-0.63: file I/O (Linux). Blocking I/O parks for GC. */
#include <errno.h>
#include <fcntl.h>
#include <sys/stat.h>
#include <unistd.h>

typedef struct { int64_t value; void *err; } uli_file_i64_result;
typedef struct { uint8_t *data; int64_t len; void *err; } uli_file_blob_result;
typedef struct { int64_t code; const char *message; } uli_file_error;

static void *uli_file_error_new(int64_t code, const char *message) {
	uli_file_error *err = (uli_file_error *)uli_alloc(sizeof(*err));
	err->code = code;
	err->message = message ? message : "file error";
	return err;
}

static void *uli_file_errno_error(void) {
	return uli_file_error_new(errno, strerror(errno));
}

static uli_file_i64_result uli_file_open_read(const char *path) {
	if (!path || !path[0]) return (uli_file_i64_result){ 0, uli_file_error_new(EINVAL, "empty file path") };
	char sp;
	uli_gc_park_begin(&sp);
	int fd = open(path, O_RDONLY);
	uli_gc_park_end();
	if (fd < 0) return (uli_file_i64_result){ 0, uli_file_errno_error() };
	return (uli_file_i64_result){ fd, NULL };
}

static uli_file_i64_result uli_file_create(const char *path) {
	if (!path || !path[0]) return (uli_file_i64_result){ 0, uli_file_error_new(EINVAL, "empty file path") };
	char sp;
	uli_gc_park_begin(&sp);
	int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0644);
	uli_gc_park_end();
	if (fd < 0) return (uli_file_i64_result){ 0, uli_file_errno_error() };
	return (uli_file_i64_result){ fd, NULL };
}

static uli_file_i64_result uli_file_read(int64_t fd, uint8_t *data, int64_t len) {
	if (len < 0) return (uli_file_i64_result){ 0, uli_file_error_new(EINVAL, "invalid read buffer") };
	if (len == 0) return (uli_file_i64_result){ 0, NULL };
	if (!data) return (uli_file_i64_result){ 0, uli_file_error_new(EINVAL, "invalid read buffer") };
	char sp;
	uli_gc_park_begin(&sp);
	ssize_t n = read((int)fd, data, (size_t)len);
	uli_gc_park_end();
	if (n < 0) return (uli_file_i64_result){ 0, uli_file_errno_error() };
	return (uli_file_i64_result){ (int64_t)n, NULL };
}

static uli_file_i64_result uli_file_write(int64_t fd, const uint8_t *data, int64_t len) {
	if (len < 0) return (uli_file_i64_result){ 0, uli_file_error_new(EINVAL, "invalid write buffer") };
	if (len == 0) return (uli_file_i64_result){ 0, NULL };
	if (!data) return (uli_file_i64_result){ 0, uli_file_error_new(EINVAL, "invalid write buffer") };
	int64_t off = 0;
	while (off < len) {
		char sp;
		uli_gc_park_begin(&sp);
		ssize_t n = write((int)fd, data + off, (size_t)(len - off));
		uli_gc_park_end();
		if (n <= 0) return (uli_file_i64_result){ off, uli_file_errno_error() };
		off += n;
	}
	return (uli_file_i64_result){ off, NULL };
}

static void *uli_file_close(int64_t fd) {
	if (fd >= 0 && close((int)fd) != 0) return uli_file_errno_error();
	return NULL;
}

static uli_file_blob_result uli_file_read_all(const char *path) {
	uli_file_i64_result opened = uli_file_open_read(path);
	if (opened.err) return (uli_file_blob_result){ NULL, 0, opened.err };
	int fd = (int)opened.value;
	struct stat st;
	char sp;
	uli_gc_park_begin(&sp);
	int src = fstat(fd, &st);
	uli_gc_park_end();
	if (src != 0) {
		void *err = uli_file_errno_error();
		close(fd);
		return (uli_file_blob_result){ NULL, 0, err };
	}
	if (!S_ISREG(st.st_mode)) {
		close(fd);
		return (uli_file_blob_result){ NULL, 0, uli_file_error_new(EINVAL, "not a regular file") };
	}
	if (st.st_size < 0) {
		close(fd);
		return (uli_file_blob_result){ NULL, 0, uli_file_error_new(EINVAL, "invalid file size") };
	}
	size_t need = (size_t)st.st_size;
	uint8_t *buf = NULL;
	if (need > 0) {
		buf = (uint8_t *)uli_alloc(need);
		size_t off = 0;
		while (off < need) {
			uli_gc_park_begin(&sp);
			ssize_t n = read(fd, buf + off, need - off);
			uli_gc_park_end();
			if (n < 0) {
				void *err = uli_file_errno_error();
				close(fd);
				return (uli_file_blob_result){ NULL, 0, err };
			}
			if (n == 0) break;
			off += (size_t)n;
		}
		need = off;
	}
	if (close(fd) != 0) return (uli_file_blob_result){ NULL, 0, uli_file_errno_error() };
	return (uli_file_blob_result){ buf, (int64_t)need, NULL };
}

static void *uli_file_write_all(const char *path, const uint8_t *data, int64_t len) {
	uli_file_i64_result opened = uli_file_create(path);
	if (opened.err) return opened.err;
	uli_file_i64_result wrote = uli_file_write(opened.value, data, len);
	void *cerr = uli_file_close(opened.value);
	if (wrote.err) return wrote.err;
	return cerr;
}
