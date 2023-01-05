#ifndef NEXEC_H
#define NEXEC_H

#define _GNU_SOURCE
#include <assert.h>
#include <sched.h>
#include <setjmp.h>
#include <stdlib.h>
#include <time.h>
#include <unistd.h>
#include <signal.h>

#define CLONE_PARENT 0x00008000

#define STAGE_SETUP  -1
#define STAGE_PARENT  0
#define STAGE_CHILD   1
#define STAGE_INIT    2
int current_stage = STAGE_SETUP;

static int syncfd = -1;

/* Assume the stack grows down, so arguments should be above it. */
struct clone_t {
	/*
	 * Reserve some space for clone() to locate arguments
	 * and retcode in this place
	 */
	char stack[4096] __attribute__((aligned(16)));
	char stack_ptr[0];

	/* There's two children. This is used to execute the different code. */
	jmp_buf *env;
	int jmpval;
};

enum sync_t {
	SYNC_USERMAP_PLS = 0x40,	/* Request parent to map our users. */
	SYNC_USERMAP_ACK = 0x41,	/* Mapping finished by the parent. */
	SYNC_RECVPID_PLS = 0x42,	/* Tell parent we're sending the PID. */
	SYNC_RECVPID_ACK = 0x43,	/* PID was correctly received by parent. */
	SYNC_GRANDCHILD = 0x44,	/* The grandchild is ready to run. */
	SYNC_CHILD_FINISH = 0x45,	/* The child or grandchild has finished. */
	SYNC_MOUNTSOURCES_PLS = 0x46,	/* Tell parent to send mount sources by SCM_RIGHTS. */
	SYNC_MOUNTSOURCES_ACK = 0x47,	/* All mount sources have been sent. */
};

struct nlconfig_t {
	char *data;

	/* Process settings. */
	uint32_t cloneflags;
};

static int child_func(void *arg){
    struct clone_t *ca = (struct clone_t *)arg;
    longjmp(*ca->env, ca->jmpval);
}

static int clone_parent(jmp_buf *env, int jmpval){
    struct clone_t ca = {
    .env = env,
    .jmpval = jmpval,
  };
  return clone(child_func, ca.stack_ptr, CLONE_PARENT | SIGCHLD, &ca);
}

#define bail(fmt, ...)                      \
  do {                                      \
    fprintf(stderr, "FATAL: " fmt ": %m\n", \
            ##__VA_ARGS__);                 \
    exit(1);                                \
  } while(0)

#define LOG_LEVEL_ERROR   1
#define LOG_LEVEL_WARNING   2
#define LOG_LEVEL_INFO   3
#define LOG_LEVEL_DEBUG   4

void log_message(int log_level, const char* message) {
  time_t t = time(NULL);
  struct tm tm = *localtime(&t);

  char log_level_str[8];
  switch (log_level) {
    case LOG_LEVEL_ERROR:   strcpy(log_level_str, "ERROR"); break;
    case LOG_LEVEL_WARNING: strcpy(log_level_str, "WARNING"); break;
    case LOG_LEVEL_INFO:    strcpy(log_level_str, "INFO"); break;
    case LOG_LEVEL_DEBUG:   strcpy(log_level_str, "DEBUG"); break;
  }

  fprintf(stdout, "[%04d-%02d-%02d %02d:%02d:%02d] [%s] %s\n",
          tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
          tm.tm_hour, tm.tm_min, tm.tm_sec,
          log_level_str, message);
}

static int update_uidmap(pid_t pid, char *map, size_t map_len){
  int fd;
  char map_path[1024];

  snprintf(map_path, sizeof(map_path), "/proc/%d/uid_map", pid);
  fd = open(map_path, O_WRONLY);
  if (fd < 0) {
    perror("open uid_map");
    return -1;
  }

  if (write(fd, map, map_len) < 0) {
    perror("write uid_map");
    close(fd);
    return -1;
  }

  close(fd);
  return 0;
}

#endif  // NEXEC_H