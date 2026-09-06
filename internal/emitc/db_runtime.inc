/* Niraluli Tamil-0.62: shared SQL vtable with a dynamically loaded SQLite backend. */
#include <dlfcn.h>

#define ULI_DB_CONN_MAGIC UINT64_C(0x4152414d4442434e)
#define ULI_DB_STMT_MAGIC UINT64_C(0x4152414d44425354)
#define ULI_DB_OK 0
#define ULI_SQLITE_OK 0
#define ULI_SQLITE_NULL 5
#define ULI_SQLITE_ROW 100
#define ULI_SQLITE_DONE 101
#define ULI_SQLITE_OPEN_READWRITE 0x00000002
#define ULI_SQLITE_OPEN_CREATE 0x00000004

typedef struct uli_db_vtable uli_db_vtable;
typedef struct {
	uint64_t magic;
	const uli_db_vtable *vtable;
	void *impl;
} uli_db_conn;
typedef struct {
	uint64_t magic;
	uli_db_conn *owner;
	void *impl;
} uli_db_stmt;

typedef struct { int64_t value; void *err; } uli_db_i64_result;
typedef struct { double value; void *err; } uli_db_f64_result;
typedef struct { int value; void *err; } uli_db_bool_result;
typedef struct { const char *value; void *err; } uli_db_str_result;
typedef struct { uint8_t *data; int64_t len; void *err; } uli_db_blob_result;
typedef struct { int64_t affected; int64_t inserted; void *err; } uli_db_exec_result;
typedef struct { int64_t code; const char *message; } uli_db_error;

struct uli_db_vtable {
	const char *name;
	int (*open)(const char *, void **);
	int (*close)(void *);
	int (*exec)(void *, const char *, int64_t *, int64_t *);
	int (*prepare)(void *, const char *, void **);
	int (*bind_null)(void *, int);
	int (*bind_i64)(void *, int, int64_t);
	int (*bind_f64)(void *, int, double);
	int (*bind_text)(void *, int, const char *, int);
	int (*bind_blob)(void *, int, const void *, int);
	int (*step)(void *, int *);
	int (*column_count)(void *);
	const char *(*column_name)(void *, int);
	int (*column_is_null)(void *, int);
	int64_t (*column_i64)(void *, int);
	double (*column_f64)(void *, int);
	const unsigned char *(*column_text)(void *, int);
	const void *(*column_blob)(void *, int);
	int (*column_bytes)(void *, int);
	int (*reset)(void *);
	int (*clear_bindings)(void *);
	int (*finalize)(void *);
	const char *(*error_message)(void *);
};

static const char *uli_db_copy_text(const char *text, size_t n) {
	char *copy = (char *)uli_alloc(n + 1);
	if (n && text) memcpy(copy, text, n);
	copy[n] = '\0';
	return copy;
}

static void *uli_db_error_new(int64_t code, const char *message) {
	uli_db_error *err = (uli_db_error *)uli_alloc(sizeof(*err));
	err->code = code;
	if (!message) message = "database error";
	err->message = uli_db_copy_text(message, strlen(message));
	return err;
}

static uli_db_conn *uli_db_conn_get(int64_t handle) {
	uli_db_conn *conn = (uli_db_conn *)(intptr_t)handle;
	if (!conn || conn->magic != ULI_DB_CONN_MAGIC || !conn->vtable || !conn->impl) return NULL;
	return conn;
}

static uli_db_stmt *uli_db_stmt_get(int64_t handle) {
	uli_db_stmt *stmt = (uli_db_stmt *)(intptr_t)handle;
	if (!stmt || stmt->magic != ULI_DB_STMT_MAGIC || !stmt->owner ||
	    stmt->owner->magic != ULI_DB_CONN_MAGIC || !stmt->impl) return NULL;
	return stmt;
}

static void *uli_db_conn_error(uli_db_conn *conn, int code) {
	const char *message = NULL;
	if (conn && conn->vtable && conn->vtable->error_message && conn->impl)
		message = conn->vtable->error_message(conn->impl);
	return uli_db_error_new(code, message);
}

/* SQLite declarations are loaded with dlsym so generated C needs no sqlite3.h
   and no build-time -lsqlite3. libsqlite3.so.0 is required only at runtime. */
typedef void (*uli_sqlite_destructor)(void *);
typedef struct {
	void *library;
	int available;
	char load_error[256];
	int (*open_v2)(const char *, void **, int, const char *);
	int (*close_fn)(void *);
	const char *(*errmsg)(void *);
	int (*exec)(void *, const char *, int (*)(void *, int, char **, char **), void *, char **);
	void (*free_fn)(void *);
	int (*changes)(void *);
	int64_t (*last_insert_rowid)(void *);
	int (*prepare_v2)(void *, const char *, int, void **, const char **);
	int (*bind_null)(void *, int);
	int (*bind_int64)(void *, int, int64_t);
	int (*bind_double)(void *, int, double);
	int (*bind_text)(void *, int, const char *, int, uli_sqlite_destructor);
	int (*bind_blob)(void *, int, const void *, int, uli_sqlite_destructor);
	int (*step)(void *);
	int (*column_count)(void *);
	const char *(*column_name)(void *, int);
	int (*column_type)(void *, int);
	int64_t (*column_int64)(void *, int);
	double (*column_double)(void *, int);
	const unsigned char *(*column_text)(void *, int);
	const void *(*column_blob)(void *, int);
	int (*column_bytes)(void *, int);
	int (*reset)(void *);
	int (*clear_bindings)(void *);
	int (*finalize)(void *);
} uli_sqlite_api;

static uli_sqlite_api uli_sqlite;
static pthread_once_t uli_sqlite_once = PTHREAD_ONCE_INIT;

static void uli_sqlite_load(void) {
	memset(&uli_sqlite, 0, sizeof uli_sqlite);
	uli_sqlite.library = dlopen("libsqlite3.so.0", RTLD_NOW | RTLD_LOCAL);
	if (!uli_sqlite.library) uli_sqlite.library = dlopen("libsqlite3.so", RTLD_NOW | RTLD_LOCAL);
	if (!uli_sqlite.library) {
		const char *why = dlerror();
		snprintf(uli_sqlite.load_error, sizeof uli_sqlite.load_error,
		         "SQLite runtime unavailable: %s", why ? why : "libsqlite3 not found");
		return;
	}
#define ULI_SQLITE_LOAD(field, symbol) do { \
	*(void **)(&uli_sqlite.field) = dlsym(uli_sqlite.library, symbol); \
	if (!uli_sqlite.field) { \
		snprintf(uli_sqlite.load_error, sizeof uli_sqlite.load_error, \
		         "SQLite runtime missing symbol %s", symbol); \
		return; \
	} \
} while (0)
	ULI_SQLITE_LOAD(open_v2, "sqlite3_open_v2");
	ULI_SQLITE_LOAD(close_fn, "sqlite3_close");
	ULI_SQLITE_LOAD(errmsg, "sqlite3_errmsg");
	ULI_SQLITE_LOAD(exec, "sqlite3_exec");
	ULI_SQLITE_LOAD(free_fn, "sqlite3_free");
	ULI_SQLITE_LOAD(changes, "sqlite3_changes");
	ULI_SQLITE_LOAD(last_insert_rowid, "sqlite3_last_insert_rowid");
	ULI_SQLITE_LOAD(prepare_v2, "sqlite3_prepare_v2");
	ULI_SQLITE_LOAD(bind_null, "sqlite3_bind_null");
	ULI_SQLITE_LOAD(bind_int64, "sqlite3_bind_int64");
	ULI_SQLITE_LOAD(bind_double, "sqlite3_bind_double");
	ULI_SQLITE_LOAD(bind_text, "sqlite3_bind_text");
	ULI_SQLITE_LOAD(bind_blob, "sqlite3_bind_blob");
	ULI_SQLITE_LOAD(step, "sqlite3_step");
	ULI_SQLITE_LOAD(column_count, "sqlite3_column_count");
	ULI_SQLITE_LOAD(column_name, "sqlite3_column_name");
	ULI_SQLITE_LOAD(column_type, "sqlite3_column_type");
	ULI_SQLITE_LOAD(column_int64, "sqlite3_column_int64");
	ULI_SQLITE_LOAD(column_double, "sqlite3_column_double");
	ULI_SQLITE_LOAD(column_text, "sqlite3_column_text");
	ULI_SQLITE_LOAD(column_blob, "sqlite3_column_blob");
	ULI_SQLITE_LOAD(column_bytes, "sqlite3_column_bytes");
	ULI_SQLITE_LOAD(reset, "sqlite3_reset");
	ULI_SQLITE_LOAD(clear_bindings, "sqlite3_clear_bindings");
	ULI_SQLITE_LOAD(finalize, "sqlite3_finalize");
#undef ULI_SQLITE_LOAD
	uli_sqlite.available = 1;
}

static int uli_sqlite_open(const char *dsn, void **out) {
	pthread_once(&uli_sqlite_once, uli_sqlite_load);
	if (!uli_sqlite.available) return -1;
	return uli_sqlite.open_v2(dsn, out,
		ULI_SQLITE_OPEN_READWRITE | ULI_SQLITE_OPEN_CREATE, NULL);
}

static int uli_sqlite_close(void *conn) { return uli_sqlite.close_fn(conn); }

static int uli_sqlite_exec(void *conn, const char *sql, int64_t *affected, int64_t *inserted) {
	char *message = NULL;
	int rc = uli_sqlite.exec(conn, sql, NULL, NULL, &message);
	if (message) uli_sqlite.free_fn(message);
	if (rc == ULI_SQLITE_OK) {
		*affected = uli_sqlite.changes(conn);
		*inserted = uli_sqlite.last_insert_rowid(conn);
	}
	return rc;
}

static int uli_sqlite_prepare(void *conn, const char *sql, void **out) {
	return uli_sqlite.prepare_v2(conn, sql, -1, out, NULL);
}
static int uli_sqlite_bind_null(void *stmt, int index) {
	return uli_sqlite.bind_null(stmt, index);
}
static int uli_sqlite_bind_i64(void *stmt, int index, int64_t value) {
	return uli_sqlite.bind_int64(stmt, index, value);
}
static int uli_sqlite_bind_f64(void *stmt, int index, double value) {
	return uli_sqlite.bind_double(stmt, index, value);
}
static int uli_sqlite_bind_text(void *stmt, int index, const char *value, int len) {
	return uli_sqlite.bind_text(stmt, index, value, len, (uli_sqlite_destructor)(intptr_t)-1);
}
static int uli_sqlite_bind_blob(void *stmt, int index, const void *value, int len) {
	return uli_sqlite.bind_blob(stmt, index, value, len, (uli_sqlite_destructor)(intptr_t)-1);
}
static int uli_sqlite_step(void *stmt, int *has_row) {
	int rc = uli_sqlite.step(stmt);
	if (rc == ULI_SQLITE_ROW) {
		*has_row = 1;
		return ULI_DB_OK;
	}
	if (rc == ULI_SQLITE_DONE) {
		*has_row = 0;
		return ULI_DB_OK;
	}
	return rc;
}
static int uli_sqlite_column_count(void *stmt) { return uli_sqlite.column_count(stmt); }
static const char *uli_sqlite_column_name(void *stmt, int index) {
	return uli_sqlite.column_name(stmt, index);
}
static int uli_sqlite_column_is_null(void *stmt, int index) {
	return uli_sqlite.column_type(stmt, index) == ULI_SQLITE_NULL;
}
static int64_t uli_sqlite_column_i64(void *stmt, int index) {
	return uli_sqlite.column_int64(stmt, index);
}
static double uli_sqlite_column_f64(void *stmt, int index) {
	return uli_sqlite.column_double(stmt, index);
}
static const unsigned char *uli_sqlite_column_text(void *stmt, int index) {
	return uli_sqlite.column_text(stmt, index);
}
static const void *uli_sqlite_column_blob(void *stmt, int index) {
	return uli_sqlite.column_blob(stmt, index);
}
static int uli_sqlite_column_bytes(void *stmt, int index) {
	return uli_sqlite.column_bytes(stmt, index);
}
static int uli_sqlite_reset(void *stmt) { return uli_sqlite.reset(stmt); }
static int uli_sqlite_clear_bindings(void *stmt) { return uli_sqlite.clear_bindings(stmt); }
static int uli_sqlite_finalize(void *stmt) { return uli_sqlite.finalize(stmt); }
static const char *uli_sqlite_error_message(void *conn) { return uli_sqlite.errmsg(conn); }

static const uli_db_vtable uli_sqlite_vtable = {
	"sqlite",
	uli_sqlite_open, uli_sqlite_close, uli_sqlite_exec, uli_sqlite_prepare,
	uli_sqlite_bind_null, uli_sqlite_bind_i64, uli_sqlite_bind_f64,
	uli_sqlite_bind_text, uli_sqlite_bind_blob, uli_sqlite_step,
	uli_sqlite_column_count, uli_sqlite_column_name, uli_sqlite_column_is_null,
	uli_sqlite_column_i64, uli_sqlite_column_f64, uli_sqlite_column_text,
	uli_sqlite_column_blob, uli_sqlite_column_bytes, uli_sqlite_reset,
	uli_sqlite_clear_bindings, uli_sqlite_finalize, uli_sqlite_error_message
};

/* ---- Tamil-0.81: PostgreSQL via libpq (dlopen; no -lpq at build time) ---- */
#define ULI_PQ_CONNECTION_OK 0
#define ULI_PQ_RES_COMMAND_OK 1
#define ULI_PQ_RES_TUPLES_OK 2
#define ULI_PQ_RES_FATAL_ERROR 7
#define ULI_PQ_MAX_PARAMS 64

typedef struct {
	void *library;
	int available;
	char load_error[256];
	void *(*connectdb)(const char *);
	int (*status)(void *);
	char *(*errorMessage)(void *);
	void (*finish)(void *);
	void *(*exec)(void *, const char *);
	void *(*execParams)(void *, const char *, int, const void *, const char *const *, const int *, const int *, int);
	int (*resultStatus)(void *);
	char *(*resErrorMessage)(void *);
	char *(*cmdTuples)(void *);
	int (*ntuples)(void *);
	int (*nfields)(void *);
	char *(*fname)(void *, int);
	char *(*getvalue)(void *, int, int);
	int (*getisnull)(void *, int, int);
	int (*getlength)(void *, int, int);
	void (*clear)(void *);
	unsigned char *(*unescapeBytea)(const unsigned char *, size_t *);
	void (*freemem)(void *);
} uli_pq_api;

static uli_pq_api uli_pq;
static pthread_once_t uli_pq_once = PTHREAD_ONCE_INIT;

static void uli_pq_load(void) {
	memset(&uli_pq, 0, sizeof uli_pq);
	uli_pq.library = dlopen("libpq.so.5", RTLD_NOW | RTLD_LOCAL);
	if (!uli_pq.library) uli_pq.library = dlopen("libpq.so", RTLD_NOW | RTLD_LOCAL);
	if (!uli_pq.library) {
		const char *why = dlerror();
		snprintf(uli_pq.load_error, sizeof uli_pq.load_error,
		         "PostgreSQL runtime unavailable: %s", why ? why : "libpq not found");
		return;
	}
#define ULI_PQ_LOAD(field, symbol) do { \
	*(void **)(&uli_pq.field) = dlsym(uli_pq.library, symbol); \
	if (!uli_pq.field) { \
		snprintf(uli_pq.load_error, sizeof uli_pq.load_error, \
		         "PostgreSQL runtime missing symbol %s", symbol); \
		return; \
	} \
} while (0)
	ULI_PQ_LOAD(connectdb, "PQconnectdb");
	ULI_PQ_LOAD(status, "PQstatus");
	ULI_PQ_LOAD(errorMessage, "PQerrorMessage");
	ULI_PQ_LOAD(finish, "PQfinish");
	ULI_PQ_LOAD(exec, "PQexec");
	ULI_PQ_LOAD(execParams, "PQexecParams");
	ULI_PQ_LOAD(resultStatus, "PQresultStatus");
	ULI_PQ_LOAD(resErrorMessage, "PQresultErrorMessage");
	ULI_PQ_LOAD(cmdTuples, "PQcmdTuples");
	ULI_PQ_LOAD(ntuples, "PQntuples");
	ULI_PQ_LOAD(nfields, "PQnfields");
	ULI_PQ_LOAD(fname, "PQfname");
	ULI_PQ_LOAD(getvalue, "PQgetvalue");
	ULI_PQ_LOAD(getisnull, "PQgetisnull");
	ULI_PQ_LOAD(getlength, "PQgetlength");
	ULI_PQ_LOAD(clear, "PQclear");
	ULI_PQ_LOAD(unescapeBytea, "PQunescapeBytea");
	ULI_PQ_LOAD(freemem, "PQfreemem");
#undef ULI_PQ_LOAD
	uli_pq.available = 1;
}

typedef struct {
	void *conn;
	char last_error[512];
} uli_pq_conn;

typedef struct {
	uli_pq_conn *owner;
	char *sql;
	char *params[ULI_PQ_MAX_PARAMS];
	int param_isnull[ULI_PQ_MAX_PARAMS];
	int nparams;
	void *result;
	int row;
	int executed;
	unsigned char *blob_cache;
	int blob_cache_len;
} uli_pq_stmt;

static void uli_pq_set_error(uli_pq_conn *c, const char *msg) {
	if (!c) return;
	snprintf(c->last_error, sizeof c->last_error, "%s", msg ? msg : "postgres error");
}

static int uli_pq_open(const char *dsn, void **out) {
	pthread_once(&uli_pq_once, uli_pq_load);
	if (!uli_pq.available) return -1;
	void *conn = uli_pq.connectdb(dsn);
	if (!conn) return -1;
	if (uli_pq.status(conn) != ULI_PQ_CONNECTION_OK) {
		uli_pq_conn *wrap = (uli_pq_conn *)uli_alloc(sizeof(*wrap));
		wrap->conn = conn;
		uli_pq_set_error(wrap, uli_pq.errorMessage(conn));
		*out = wrap;
		return -1;
	}
	uli_pq_conn *wrap = (uli_pq_conn *)uli_alloc(sizeof(*wrap));
	wrap->conn = conn;
	wrap->last_error[0] = '\0';
	*out = wrap;
	return ULI_DB_OK;
}

static int uli_pq_close(void *conn) {
	uli_pq_conn *c = (uli_pq_conn *)conn;
	if (!c || !c->conn) return ULI_DB_OK;
	uli_pq.finish(c->conn);
	c->conn = NULL;
	return ULI_DB_OK;
}

static int uli_pq_exec(void *conn, const char *sql, int64_t *affected, int64_t *inserted) {
	uli_pq_conn *c = (uli_pq_conn *)conn;
	void *res = uli_pq.exec(c->conn, sql);
	if (!res) {
		uli_pq_set_error(c, uli_pq.errorMessage(c->conn));
		return -1;
	}
	int st = uli_pq.resultStatus(res);
	if (st != ULI_PQ_RES_COMMAND_OK && st != ULI_PQ_RES_TUPLES_OK) {
		uli_pq_set_error(c, uli_pq.resErrorMessage(res));
		uli_pq.clear(res);
		return st;
	}
	*affected = 0;
	*inserted = 0;
	const char *ct = uli_pq.cmdTuples(res);
	if (ct && *ct) *affected = (int64_t)atoll(ct);
	uli_pq.clear(res);
	return ULI_DB_OK;
}

static int uli_pq_count_params(const char *sql) {
	int max = 0;
	for (const char *p = sql; *p; p++) {
		if (*p == '$' && p[1] >= '1' && p[1] <= '9') {
			int n = 0;
			const char *q = p + 1;
			while (*q >= '0' && *q <= '9') { n = n * 10 + (*q - '0'); q++; }
			if (n > max) max = n;
		}
	}
	return max;
}

static int uli_pq_prepare(void *conn, const char *sql, void **out) {
	uli_pq_conn *c = (uli_pq_conn *)conn;
	if (!sql) return -1;
	uli_pq_stmt *st = (uli_pq_stmt *)uli_alloc(sizeof(*st));
	memset(st, 0, sizeof(*st));
	st->owner = c;
	size_t n = strlen(sql);
	st->sql = (char *)uli_alloc(n + 1);
	memcpy(st->sql, sql, n + 1);
	st->nparams = uli_pq_count_params(sql);
	if (st->nparams > ULI_PQ_MAX_PARAMS) {
		uli_pq_set_error(c, "too many parameters");
		return -1;
	}
	for (int i = 0; i < ULI_PQ_MAX_PARAMS; i++) st->param_isnull[i] = 1;
	*out = st;
	return ULI_DB_OK;
}

static int uli_pq_bind_null(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (index < 1 || index > ULI_PQ_MAX_PARAMS) return -1;
	st->params[index - 1] = NULL;
	st->param_isnull[index - 1] = 1;
	if (index > st->nparams) st->nparams = index;
	return ULI_DB_OK;
}

static int uli_pq_bind_i64(void *stmt, int index, int64_t value) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (index < 1 || index > ULI_PQ_MAX_PARAMS) return -1;
	char buf[32];
	snprintf(buf, sizeof buf, "%lld", (long long)value);
	size_t n = strlen(buf);
	char *copy = (char *)uli_alloc(n + 1);
	memcpy(copy, buf, n + 1);
	st->params[index - 1] = copy;
	st->param_isnull[index - 1] = 0;
	if (index > st->nparams) st->nparams = index;
	return ULI_DB_OK;
}

static int uli_pq_bind_f64(void *stmt, int index, double value) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (index < 1 || index > ULI_PQ_MAX_PARAMS) return -1;
	char buf[64];
	snprintf(buf, sizeof buf, "%.17g", value);
	size_t n = strlen(buf);
	char *copy = (char *)uli_alloc(n + 1);
	memcpy(copy, buf, n + 1);
	st->params[index - 1] = copy;
	st->param_isnull[index - 1] = 0;
	if (index > st->nparams) st->nparams = index;
	return ULI_DB_OK;
}

static int uli_pq_bind_text(void *stmt, int index, const char *value, int len) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (index < 1 || index > ULI_PQ_MAX_PARAMS) return -1;
	if (len < 0) len = value ? (int)strlen(value) : 0;
	char *copy = (char *)uli_alloc((size_t)len + 1);
	if (len && value) memcpy(copy, value, (size_t)len);
	copy[len] = '\0';
	st->params[index - 1] = copy;
	st->param_isnull[index - 1] = 0;
	if (index > st->nparams) st->nparams = index;
	return ULI_DB_OK;
}

static int uli_pq_bind_blob(void *stmt, int index, const void *value, int len) {
	/* Store as hex escape \\x... for text-format bytea. */
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (index < 1 || index > ULI_PQ_MAX_PARAMS || len < 0) return -1;
	const unsigned char *bytes = (const unsigned char *)value;
	char *copy = (char *)uli_alloc((size_t)len * 2 + 3);
	copy[0] = '\\';
	copy[1] = 'x';
	static const char hex[] = "0123456789abcdef";
	for (int i = 0; i < len; i++) {
		copy[2 + i * 2] = hex[bytes[i] >> 4];
		copy[3 + i * 2] = hex[bytes[i] & 0xf];
	}
	copy[2 + len * 2] = '\0';
	st->params[index - 1] = copy;
	st->param_isnull[index - 1] = 0;
	if (index > st->nparams) st->nparams = index;
	return ULI_DB_OK;
}

static int uli_pq_step(void *stmt, int *has_row) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	*has_row = 0;
	if (!st->executed) {
		const char *values[ULI_PQ_MAX_PARAMS];
		for (int i = 0; i < st->nparams; i++) {
			values[i] = st->param_isnull[i] ? NULL : st->params[i];
		}
		st->result = uli_pq.execParams(st->owner->conn, st->sql, st->nparams, NULL,
			values, NULL, NULL, 0);
		st->executed = 1;
		st->row = -1;
		if (!st->result) {
			uli_pq_set_error(st->owner, uli_pq.errorMessage(st->owner->conn));
			return -1;
		}
		int rs = uli_pq.resultStatus(st->result);
		if (rs != ULI_PQ_RES_COMMAND_OK && rs != ULI_PQ_RES_TUPLES_OK) {
			uli_pq_set_error(st->owner, uli_pq.resErrorMessage(st->result));
			return rs;
		}
	}
	if (!st->result) return ULI_DB_OK;
	int nrows = uli_pq.ntuples(st->result);
	st->row++;
	if (st->row < nrows) {
		*has_row = 1;
		return ULI_DB_OK;
	}
	return ULI_DB_OK;
}

static int uli_pq_column_count(void *stmt) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	return st->result ? uli_pq.nfields(st->result) : 0;
}
static const char *uli_pq_column_name(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	return st->result ? uli_pq.fname(st->result, index) : "";
}
static int uli_pq_column_is_null(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (!st->result || st->row < 0) return 1;
	return uli_pq.getisnull(st->result, st->row, index);
}
static int64_t uli_pq_column_i64(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (!st->result || st->row < 0 || uli_pq.getisnull(st->result, st->row, index)) return 0;
	return (int64_t)atoll(uli_pq.getvalue(st->result, st->row, index));
}
static double uli_pq_column_f64(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (!st->result || st->row < 0 || uli_pq.getisnull(st->result, st->row, index)) return 0;
	return atof(uli_pq.getvalue(st->result, st->row, index));
}
static const unsigned char *uli_pq_column_text(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (!st->result || st->row < 0 || uli_pq.getisnull(st->result, st->row, index))
		return (const unsigned char *)"";
	return (const unsigned char *)uli_pq.getvalue(st->result, st->row, index);
}
static const void *uli_pq_column_blob(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	st->blob_cache = NULL;
	st->blob_cache_len = 0;
	if (!st->result || st->row < 0 || uli_pq.getisnull(st->result, st->row, index)) return "";
	const char *raw = uli_pq.getvalue(st->result, st->row, index);
	size_t out_len = 0;
	unsigned char *decoded = uli_pq.unescapeBytea((const unsigned char *)raw, &out_len);
	if (!decoded) {
		st->blob_cache_len = uli_pq.getlength(st->result, st->row, index);
		return raw;
	}
	unsigned char *copy = (unsigned char *)uli_alloc(out_len ? out_len : 1);
	if (out_len) memcpy(copy, decoded, out_len);
	uli_pq.freemem(decoded);
	st->blob_cache = copy;
	st->blob_cache_len = (int)out_len;
	return copy;
}
static int uli_pq_column_bytes(void *stmt, int index) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (st->blob_cache || st->blob_cache_len > 0) return st->blob_cache_len;
	if (!st->result || st->row < 0 || uli_pq.getisnull(st->result, st->row, index)) return 0;
	const char *raw = uli_pq.getvalue(st->result, st->row, index);
	size_t out_len = 0;
	unsigned char *decoded = uli_pq.unescapeBytea((const unsigned char *)raw, &out_len);
	if (decoded) {
		uli_pq.freemem(decoded);
		return (int)out_len;
	}
	return uli_pq.getlength(st->result, st->row, index);
}
static int uli_pq_reset(void *stmt) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (st->result) { uli_pq.clear(st->result); st->result = NULL; }
	st->executed = 0;
	st->row = -1;
	return ULI_DB_OK;
}
static int uli_pq_clear_bindings(void *stmt) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	for (int i = 0; i < ULI_PQ_MAX_PARAMS; i++) {
		st->params[i] = NULL;
		st->param_isnull[i] = 1;
	}
	return ULI_DB_OK;
}
static int uli_pq_finalize(void *stmt) {
	uli_pq_stmt *st = (uli_pq_stmt *)stmt;
	if (st->result) { uli_pq.clear(st->result); st->result = NULL; }
	return ULI_DB_OK;
}
static const char *uli_pq_error_message(void *conn) {
	uli_pq_conn *c = (uli_pq_conn *)conn;
	if (c && c->last_error[0]) return c->last_error;
	if (c && c->conn) return uli_pq.errorMessage(c->conn);
	return uli_pq.load_error[0] ? uli_pq.load_error : "postgres error";
}

static const uli_db_vtable uli_pq_vtable = {
	"postgres",
	uli_pq_open, uli_pq_close, uli_pq_exec, uli_pq_prepare,
	uli_pq_bind_null, uli_pq_bind_i64, uli_pq_bind_f64,
	uli_pq_bind_text, uli_pq_bind_blob, uli_pq_step,
	uli_pq_column_count, uli_pq_column_name, uli_pq_column_is_null,
	uli_pq_column_i64, uli_pq_column_f64, uli_pq_column_text,
	uli_pq_column_blob, uli_pq_column_bytes, uli_pq_reset,
	uli_pq_clear_bindings, uli_pq_finalize, uli_pq_error_message
};

static const uli_db_vtable *uli_db_backend(const char *driver) {
	if (driver && (strcmp(driver, "sqlite") == 0 || strcmp(driver, "sqlite3") == 0))
		return &uli_sqlite_vtable;
	if (driver && (strcmp(driver, "postgres") == 0 || strcmp(driver, "postgresql") == 0))
		return &uli_pq_vtable;
	return NULL;
}

static uli_db_i64_result uli_db_open(const char *driver, const char *dsn) {
	const uli_db_vtable *vtable = uli_db_backend(driver);
	if (!vtable)
		return (uli_db_i64_result){ 0, uli_db_error_new(-2, "unsupported database driver") };
	if (!dsn || !*dsn)
		return (uli_db_i64_result){ 0, uli_db_error_new(-3, "empty database address") };
	void *impl = NULL;
	char sp;
	uli_gc_park_begin(&sp);
	int rc = vtable->open(dsn, &impl);
	uli_gc_park_end();
	if (rc != ULI_DB_OK) {
		const char *message = NULL;
		if (impl && vtable->error_message) message = vtable->error_message(impl);
		if (!message && strcmp(vtable->name, "sqlite") == 0 && uli_sqlite.load_error[0])
			message = uli_sqlite.load_error;
		if (!message && strcmp(vtable->name, "postgres") == 0 && uli_pq.load_error[0])
			message = uli_pq.load_error;
		void *err = uli_db_error_new(rc, message);
		if (impl) vtable->close(impl);
		return (uli_db_i64_result){ 0, err };
	}
	uli_db_conn *conn = (uli_db_conn *)uli_alloc(sizeof(*conn));
	conn->magic = ULI_DB_CONN_MAGIC;
	conn->vtable = vtable;
	conn->impl = impl;
	return (uli_db_i64_result){ (int64_t)(intptr_t)conn, NULL };
}

static void *uli_db_close(int64_t handle) {
	volatile uli_db_conn *root = uli_db_conn_get(handle);
	if (!root) return uli_db_error_new(-4, "invalid or closed database handle");
	char sp;
	uli_gc_park_begin(&sp);
	int rc = root->vtable->close(root->impl);
	uli_gc_park_end();
	if (rc != ULI_DB_OK) return uli_db_conn_error((uli_db_conn *)root, rc);
	root->impl = NULL;
	root->magic = 0;
	return NULL;
}

static uli_db_exec_result uli_db_exec(int64_t handle, const char *sql) {
	volatile uli_db_conn *root = uli_db_conn_get(handle);
	if (!root) return (uli_db_exec_result){ 0, 0, uli_db_error_new(-4, "invalid or closed database handle") };
	if (!sql) return (uli_db_exec_result){ 0, 0, uli_db_error_new(-5, "nil SQL text") };
	int64_t affected = 0, inserted = 0;
	char sp;
	uli_gc_park_begin(&sp);
	int rc = root->vtable->exec(root->impl, sql, &affected, &inserted);
	uli_gc_park_end();
	if (rc != ULI_DB_OK)
		return (uli_db_exec_result){ 0, 0, uli_db_conn_error((uli_db_conn *)root, rc) };
	return (uli_db_exec_result){ affected, inserted, NULL };
}

static uli_db_i64_result uli_db_prepare(int64_t handle, const char *sql) {
	volatile uli_db_conn *root = uli_db_conn_get(handle);
	if (!root) return (uli_db_i64_result){ 0, uli_db_error_new(-4, "invalid or closed database handle") };
	void *impl = NULL;
	char sp;
	uli_gc_park_begin(&sp);
	int rc = root->vtable->prepare(root->impl, sql, &impl);
	uli_gc_park_end();
	if (rc != ULI_DB_OK)
		return (uli_db_i64_result){ 0, uli_db_conn_error((uli_db_conn *)root, rc) };
	uli_db_stmt *stmt = (uli_db_stmt *)uli_alloc(sizeof(*stmt));
	stmt->magic = ULI_DB_STMT_MAGIC;
	stmt->owner = (uli_db_conn *)root;
	stmt->impl = impl;
	return (uli_db_i64_result){ (int64_t)(intptr_t)stmt, NULL };
}

static void *uli_db_bind_null(int64_t handle, int64_t index) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return uli_db_error_new(-6, "invalid or finalized statement handle");
	int rc = root->owner->vtable->bind_null(root->impl, (int)index);
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(root->owner, rc);
}
static void *uli_db_bind_i64(int64_t handle, int64_t index, int64_t value) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return uli_db_error_new(-6, "invalid or finalized statement handle");
	int rc = root->owner->vtable->bind_i64(root->impl, (int)index, value);
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(root->owner, rc);
}
static void *uli_db_bind_f64(int64_t handle, int64_t index, double value) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return uli_db_error_new(-6, "invalid or finalized statement handle");
	int rc = root->owner->vtable->bind_f64(root->impl, (int)index, value);
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(root->owner, rc);
}
static void *uli_db_bind_text(int64_t handle, int64_t index, const char *value) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return uli_db_error_new(-6, "invalid or finalized statement handle");
	size_t n = value ? strlen(value) : 0;
	if (n > INT32_MAX) return uli_db_error_new(-7, "text parameter too large");
	int rc = root->owner->vtable->bind_text(root->impl, (int)index, value ? value : "", (int)n);
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(root->owner, rc);
}
static void *uli_db_bind_blob(int64_t handle, int64_t index, const uint8_t *data, int64_t len) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return uli_db_error_new(-6, "invalid or finalized statement handle");
	if (len < 0 || len > INT32_MAX) return uli_db_error_new(-7, "blob parameter too large");
	static const uint8_t empty_blob = 0;
	const uint8_t *value = data ? data : &empty_blob;
	int rc = root->owner->vtable->bind_blob(root->impl, (int)index, value, (int)len);
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(root->owner, rc);
}

static uli_db_bool_result uli_db_step(int64_t handle) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return (uli_db_bool_result){ 0, uli_db_error_new(-6, "invalid or finalized statement handle") };
	char sp;
	uli_gc_park_begin(&sp);
	int has_row = 0;
	int rc = root->owner->vtable->step(root->impl, &has_row);
	uli_gc_park_end();
	if (rc == ULI_DB_OK) return (uli_db_bool_result){ has_row, NULL };
	return (uli_db_bool_result){ 0, uli_db_conn_error(root->owner, rc) };
}

static int uli_db_column_valid(uli_db_stmt *stmt, int64_t index) {
	return stmt && index >= 0 && index < stmt->owner->vtable->column_count(stmt->impl);
}

static uli_db_i64_result uli_db_column_count(int64_t handle) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt)
		return (uli_db_i64_result){ 0, uli_db_error_new(-6, "invalid or finalized statement handle") };
	return (uli_db_i64_result){ stmt->owner->vtable->column_count(stmt->impl), NULL };
}

static uli_db_str_result uli_db_column_name(int64_t handle, int64_t index) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return (uli_db_str_result){ "", uli_db_error_new(-6, "invalid or finalized statement handle") };
	if (!uli_db_column_valid(stmt, index))
		return (uli_db_str_result){ "", uli_db_error_new(-8, "column index out of range") };
	const char *name = stmt->owner->vtable->column_name(stmt->impl, (int)index);
	return (uli_db_str_result){ uli_db_copy_text(name ? name : "", name ? strlen(name) : 0), NULL };
}

static uli_db_bool_result uli_db_column_is_null(int64_t handle, int64_t index) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return (uli_db_bool_result){ 0, uli_db_error_new(-6, "invalid or finalized statement handle") };
	if (!uli_db_column_valid(stmt, index))
		return (uli_db_bool_result){ 0, uli_db_error_new(-8, "column index out of range") };
	return (uli_db_bool_result){
		stmt->owner->vtable->column_is_null(stmt->impl, (int)index), NULL
	};
}

static uli_db_i64_result uli_db_column_i64(int64_t handle, int64_t index) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return (uli_db_i64_result){ 0, uli_db_error_new(-6, "invalid or finalized statement handle") };
	if (!uli_db_column_valid(stmt, index))
		return (uli_db_i64_result){ 0, uli_db_error_new(-8, "column index out of range") };
	return (uli_db_i64_result){ stmt->owner->vtable->column_i64(stmt->impl, (int)index), NULL };
}

static uli_db_f64_result uli_db_column_f64(int64_t handle, int64_t index) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return (uli_db_f64_result){ 0, uli_db_error_new(-6, "invalid or finalized statement handle") };
	if (!uli_db_column_valid(stmt, index))
		return (uli_db_f64_result){ 0, uli_db_error_new(-8, "column index out of range") };
	return (uli_db_f64_result){ stmt->owner->vtable->column_f64(stmt->impl, (int)index), NULL };
}

static uli_db_str_result uli_db_column_text(int64_t handle, int64_t index) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return (uli_db_str_result){ "", uli_db_error_new(-6, "invalid or finalized statement handle") };
	if (!uli_db_column_valid(stmt, index))
		return (uli_db_str_result){ "", uli_db_error_new(-8, "column index out of range") };
	const unsigned char *text = stmt->owner->vtable->column_text(stmt->impl, (int)index);
	int n = stmt->owner->vtable->column_bytes(stmt->impl, (int)index);
	return (uli_db_str_result){ uli_db_copy_text((const char *)text, text ? (size_t)n : 0), NULL };
}

static uli_db_blob_result uli_db_column_blob(int64_t handle, int64_t index) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return (uli_db_blob_result){ NULL, 0, uli_db_error_new(-6, "invalid or finalized statement handle") };
	if (!uli_db_column_valid(stmt, index))
		return (uli_db_blob_result){ NULL, 0, uli_db_error_new(-8, "column index out of range") };
	const void *data = stmt->owner->vtable->column_blob(stmt->impl, (int)index);
	int n = stmt->owner->vtable->column_bytes(stmt->impl, (int)index);
	if (!data || n <= 0) return (uli_db_blob_result){ NULL, 0, NULL };
	uint8_t *copy = (uint8_t *)uli_alloc((size_t)n);
	memcpy(copy, data, (size_t)n);
	return (uli_db_blob_result){ copy, n, NULL };
}

static void *uli_db_reset(int64_t handle) {
	uli_db_stmt *stmt = uli_db_stmt_get(handle);
	if (!stmt) return uli_db_error_new(-6, "invalid or finalized statement handle");
	int rc = stmt->owner->vtable->reset(stmt->impl);
	if (rc != ULI_DB_OK) return uli_db_conn_error(stmt->owner, rc);
	rc = stmt->owner->vtable->clear_bindings(stmt->impl);
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(stmt->owner, rc);
}

static void *uli_db_finalize(int64_t handle) {
	volatile uli_db_stmt *root = uli_db_stmt_get(handle);
	if (!root) return uli_db_error_new(-6, "invalid or finalized statement handle");
	uli_db_conn *owner = root->owner;
	char sp;
	uli_gc_park_begin(&sp);
	int rc = owner->vtable->finalize(root->impl);
	uli_gc_park_end();
	root->impl = NULL;
	root->magic = 0;
	return rc == ULI_DB_OK ? NULL : uli_db_conn_error(owner, rc);
}
