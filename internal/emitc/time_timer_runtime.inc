/* Niraluli Tamil-0.79: timers/tickers (needs channel runtime above). */
typedef struct uli_timer {
	uli_chan *ch;
	int64_t delay;
	int64_t period;
	volatile int stopped;
	volatile int fired;
	struct uli_timer *next;
} uli_timer;

static pthread_mutex_t uli_timer_mu = PTHREAD_MUTEX_INITIALIZER;
static uli_timer *uli_timers;

static void *uli_timer_thread(void *arg) {
	uli_timer *t = (uli_timer *)arg;
	uli_time_sleep(t->delay);
	if (t->period == 0) {
		if (!t->stopped) {
			int64_t now = uli_time_now_unix_nano();
			(void)uli_chan_try_send(t->ch, &now);
			t->fired = 1;
		}
		return NULL;
	}
	for (;;) {
		if (t->stopped) return NULL;
		int64_t now = uli_time_now_unix_nano();
		(void)uli_chan_try_send(t->ch, &now);
		uli_time_sleep(t->period);
	}
}

static uli_chan *uli_time_start_timer(int64_t delay, int64_t period) {
	if (delay < 0) delay = 0;
	if (period < 0) period = 0;
	uli_chan *ch = uli_chan_make((int)sizeof(int64_t), 1);
	uli_timer *t = (uli_timer *)uli_alloc(sizeof(*t));
	t->ch = ch;
	t->delay = delay;
	t->period = period;
	t->stopped = 0;
	t->fired = 0;
	pthread_mutex_lock(&uli_timer_mu);
	t->next = uli_timers;
	uli_timers = t;
	pthread_mutex_unlock(&uli_timer_mu);
	pthread_t th;
	if (pthread_create(&th, NULL, uli_timer_thread, t) != 0) abort();
	pthread_detach(th);
	return ch;
}

static uli_chan *uli_time_after(int64_t nsec) {
	return uli_time_start_timer(nsec, 0);
}

static uli_chan *uli_time_tick(int64_t nsec) {
	if (nsec <= 0) nsec = 1;
	return uli_time_start_timer(nsec, nsec);
}

static int uli_time_stop(uli_chan *ch) {
	if (!ch) return 0;
	pthread_mutex_lock(&uli_timer_mu);
	uli_timer *t = uli_timers;
	while (t) {
		if (t->ch == ch) {
			int prevented = 0;
			if (!t->stopped) {
				t->stopped = 1;
				if (t->period == 0) prevented = !t->fired;
				else prevented = 1;
			}
			pthread_mutex_unlock(&uli_timer_mu);
			return prevented;
		}
		t = t->next;
	}
	pthread_mutex_unlock(&uli_timer_mu);
	return 0;
}
