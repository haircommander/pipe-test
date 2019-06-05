#define _GNU_SOURCE
#include <string.h>
#include <glib.h>
#include <glib-unix.h>
#include "utils.h"

static ssize_t write_all(int fd, const void *buf, size_t count)
{
	size_t remaining = count;
	const char *p = buf;
	ssize_t res;

	while (remaining > 0) {
		do {
			res = write(fd, p, remaining);
		} while (res == -1 && errno == EINTR);

		if (res <= 0)
			return -1;

		remaining -= res;
		p += res;
	}

	return count;
}

static char *escape_json_string(const char *str)
{
	GString *escaped;
	const char *p;

	p = str;
	escaped = g_string_sized_new(strlen(str));

	while (*p != 0) {
		char c = *p++;
		if (c == '\\' || c == '"') {
			g_string_append_c(escaped, '\\');
			g_string_append_c(escaped, c);
		} else if (c == '\n') {
			g_string_append_printf(escaped, "\\n");
		} else if (c == '\t') {
			g_string_append_printf(escaped, "\\t");
		} else if ((c > 0 && c < 0x1f) || c == 0x7f) {
			g_string_append_printf(escaped, "\\u00%02x", (guint)c);
		} else {
			g_string_append_c(escaped, c);
		}
	}

	return g_string_free(escaped, FALSE);
}

static void write_sync_fd(int sync_pipe_fd, int res, const char *message)
{
	_cleanup_free_ char *escaped_message = NULL;
	_cleanup_free_ char *json = NULL;
	const char *res_key;
	ssize_t len;

	if (sync_pipe_fd == -1)
		return;

	res_key = "data";

	if (message) {
		escaped_message = escape_json_string(message);
		json = g_strdup_printf("{\"%s\": %d, \"message\": \"%s\"}\n", res_key, res, escaped_message);
	} else {
		json = g_strdup_printf("{\"%s\": %d}\n", res_key, res);
	}

	len = strlen(json);
	ndebugf("sending: %s", json);

	if (write_all(sync_pipe_fd, json, len) != len) {
		pexit("Unable to send container stderr message to parent");
	}
}

static int get_pipe_fd_from_env(const char *envname)
{
	char *pipe_str, *endptr;
	int pipe_fd;

	pipe_str = getenv(envname);
	if (pipe_str == NULL)
		return -1;

	errno = 0;
	pipe_fd = strtol(pipe_str, &endptr, 10);
	if (errno != 0 || *endptr != '\0')
		pexitf("unable to parse %s", envname);
	if (fcntl(pipe_fd, F_SETFD, FD_CLOEXEC) == -1)
		pexitf("unable to make %s CLOEXEC", envname);

	return pipe_fd;
}

int main() {
	int main_pid = fork();
	if (main_pid < 0) {
		pexit("Failed to fork the create command");
	} else if (main_pid != 0) {
		return EXIT_SUCCESS;
	}
	
	_cleanup_close_ int sync_pipe_fd = get_pipe_fd_from_env("_OCI_SYNCPIPE");
	//_cleanup_close_ int end_pipe_fd = get_pipe_fd_from_env("_OCI_ENDPIPE");

	write_sync_fd(sync_pipe_fd, 0, 0);
	write_sync_fd(sync_pipe_fd, 0, 0);
	
	return EXIT_SUCCESS;
}
