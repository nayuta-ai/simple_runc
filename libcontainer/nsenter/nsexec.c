#define _GNU_SOURCE
#include "nsexec.h"

#include <fcntl.h>
#include <linux/limits.h>
#include <linux/netlink.h>
#include <sched.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <sys/prctl.h>
#include <sys/socket.h>
#include <unistd.h>

#include "namespace.h"

extern char *escape_json_string(char *str);

/*
 * Log levels are the same as in logrus.
 */
#define PANIC 0
#define FATAL 1
#define ERROR 2
#define WARNING 3
#define INFO 4
#define DEBUG 5
#define TRACE 6

static const char *level_str[] = {"panic", "fatal", "error", "warning",
                                  "info",  "debug", "trace"};

static int logfd = -1;
static int loglevel = DEBUG;

void write_log(int level, const char *format, ...) {
  char *message = NULL, *stage = NULL, *json = NULL;
  va_list args;
  int ret;

  if (logfd < 0 || level > loglevel) {
    goto out;
  }

  va_start(args, format);
  ret = vasprintf(&message, format, args);
  va_end(args);
  if (ret < 0) {
    message = NULL;
    goto out;
  }

  message = escape_json_string(message);

  if (current_stage == STAGE_SETUP) {
    stage = strdup("nsexec");
    if (stage == NULL) goto out;
  } else {
    ret = asprintf(&stage, "nsexec-%d", current_stage);
    if (ret < 0) {
      stage = NULL;
      goto out;
    }
  }
  ret = asprintf(&json, "{\"level\":\"%s\", \"msg\": \"%s[%d]: %s\"}\n",
                 level_str[level], stage, getpid(), message);
  if (ret < 0) {
    json = NULL;
    goto out;
  }
  /* This logging is on a best-effort basis. In case of a short or failed
   * write there is nothing we can do, so just ignore write() errors.
   */
  ssize_t __attribute__((unused)) __res = write(logfd, json, ret);

out:
  free(message);
  free(stage);
  free(json);
}

#define bail(fmt, ...)                                        \
  do {                                                        \
    if (logfd < 0)                                            \
      fprintf(stderr, "FATAL: " fmt ": %m\n", ##__VA_ARGS__); \
    else                                                      \
      write_log(DEBUG, fmt ": %m");                           \
    exit(1);                                                  \
  } while (0)

/*
 * Use the raw syscall for versions of glibc which don't include a function for
 * it, namely (glibc 2.12).
 */
#if __GLIBC__ == 2 && __GLIBC_MINOR__ < 14
#define _GNU_SOURCE
#include "syscall.h"
#if !defined(SYS_setns) && defined(__NR_setns)
#define SYS_setns __NR_setns
#endif

#ifndef SYS_setns
#error "setns(2) syscall not supported by glibc version"
#endif

int setns(int fd, int nstype) { return syscall(SYS_setns, fd, nstype); }
#endif

/* Returns the clone(2) flag for a namespace, given the name of a namespace. */
static int nsflag(char *name) {
  if (!strcmp(name, "cgroup"))
    return CLONE_NEWCGROUP;
  else if (!strcmp(name, "ipc"))
    return CLONE_NEWIPC;
  else if (!strcmp(name, "mnt"))
    return CLONE_NEWNS;
  else if (!strcmp(name, "net"))
    return CLONE_NEWNET;
  else if (!strcmp(name, "pid"))
    return CLONE_NEWPID;
  else if (!strcmp(name, "user"))
    return CLONE_NEWUSER;
  else if (!strcmp(name, "uts"))
    return CLONE_NEWUTS;

  /* If we don't recognise a name, fallback to 0. */
  return 0;
}

static inline int sane_kill(pid_t pid, int signum) {
  if (pid > 0)
    return kill(pid, signum);
  else
    return 0;
}

static int getenv_int(const char *name) {
  char *val, *endptr;
  int ret;

  val = getenv(name);
  /* Treat empty value as unset variable. */
  if (val == NULL || *val == '\0') return -ENOENT;

  ret = strtol(val, &endptr, 10);
  if (val == endptr || *endptr != '\0')
    bail("unable to parse %s=%s", name, val);
  /*
   * Sanity check: this must be a non-negative number.
   */
  if (ret < 0) bail("bad value for %s=%s (%d)", name, val, ret);

  return ret;
}

/*
 * Sets up logging by getting log fd and log level from the environment,
 * if available.
 */
static void setup_logpipe(void) {
  int i;
  i = getenv_int("_LIBCONTAINER_LOGPIPE");
  if (i < 0) {
    /* We are not runc init, or log pipe was not provided. */
    return;
  }
  logfd = i;

  i = getenv_int("_LIBCONTAINER_LOGLEVEL");
  if (i < 0) return;
  loglevel = i;
}

static void nl_parse(int fd, struct nlconfig_t *config) {
  size_t len, size;
  struct nlmsghdr hdr;
  char *current, *data;

  /* Retrieve the netlink header. */
  len = read(fd, &hdr, NLMSG_HDRLEN);
  if (len != NLMSG_HDRLEN) bail("invalid netlink header length %zu", len);
  if (hdr.nlmsg_type == NLMSG_ERROR) bail("failed to read netlink message");

  if (hdr.nlmsg_type != INIT_MSG)
    bail("unexpected msg type %d", hdr.nlmsg_type);

  /* Retrieve data. */
  size = NLMSG_PAYLOAD(&hdr, 0);
  data = (char *)malloc(size);
  current = data;

  if (!data)
    bail("failed to allocate %zu bytes of memory for nl_payload", size);

  len = read(fd, data, size);
  if (len != size)
    bail("failed to read netlink payload, %zu != %zu", len, size);

  /* Parse the netlink payload. */
  config->data = data;
  while (current < data + size) {
    struct nlattr *nlattr = (struct nlattr *)current;
    size_t payload_len = nlattr->nla_len - NLA_HDRLEN;

    /* Advance to payload. */
    current += NLA_HDRLEN;

    /* Handle payload. */
    switch (nlattr->nla_type) {
      case CLONE_FLAGS_ATTR:
        config->cloneflags = readint32(current);
        break;
      case NS_PATHS_ATTR:
        config->namespaces = current;
        config->namespaces_len = payload_len;
        break;
      default:
        bail("unknown netlink message type %d", nlattr->nla_type);
    }

    current += NLA_ALIGN(payload_len);
  }
}

void nl_free(struct nlconfig_t *config) { free(config->data); }

void join_namespaces(char *nslist) {
  int num = 0, i;
  char *saveptr = NULL;
  char *namespace = strtok_r(nslist, ",", &saveptr);
  struct namespace_t {
    int fd;
    char type[PATH_MAX];
    char path[PATH_MAX];
  } *namespaces = NULL;

  if (!namespace || !strlen(namespace) || !strlen(nslist))
    bail("ns paths are empty");

  /*
   * We have to open the file descriptors first, since after
   * we join the mnt namespace we might no longer be able to
   * access the paths.
   */
  do {
    int fd;
    char *path;
    struct namespace_t *ns;

    /* Resize the namespace array. */
    namespaces = realloc(namespaces, ++num * sizeof(struct namespace_t));
    if (!namespaces) bail("failed to reallocate namespace array");
    ns = &namespaces[num - 1];

    /* Split 'ns:path'. */
    path = strstr(namespace, ":");
    if (!path) bail("failed to parse %s", namespace);
    *path++ = '\0';

    fd = open(path, O_RDONLY);
    if (fd < 0) bail("failed to open %s", path);

    ns->fd = fd;
    strncpy(ns->type, namespace, PATH_MAX - 1);
    strncpy(ns->path, path, PATH_MAX - 1);
    ns->path[PATH_MAX - 1] = '\0';
  } while ((namespace = strtok_r(NULL, ",", &saveptr)) != NULL);

  /*
   * The ordering in which we join namespaces is important. We should
   * always join the user namespace *first*. This is all guaranteed
   * from the container_linux.go side of this, so we're just going to
   * follow the order given to us.
   */

  for (i = 0; i < num; i++) {
    struct namespace_t *ns = &namespaces[i];
    int flag = nsflag(ns->type);

    write_log(DEBUG, "setns(%#x) into %s namespace (with path %s)", flag,
              ns->type, ns->path);
    if (setns(ns->fd, flag) < 0)
      bail("failed to setns into %s namespace", ns->type);

    close(ns->fd);
  }

  free(namespaces);
}

void try_unshare(int flags, const char *msg) {
  write_log(DEBUG, msg);
  /*
   * Kernels prior to v4.3 may return EINVAL on unshare when another process
   * reads runc's /proc/$PID/status or /proc/$PID/maps. To work around this,
   * retry on EINVAL a few times.
   */
  int retries = 5;
  for (; retries > 0; retries--) {
    if (unshare(flags) == 0) {
      return;
    }
    if (errno != EINVAL) break;
  }
  bail("failed to unshare %s", msg);
}

// nsenter.go call nsexec function for creating containers.
void nsexec(void) {
  int pipenum;
  jmp_buf env;
  struct nlconfig_t config = {0};
  char map[] = "0 100000 100000\n";
  int sync_child_pipe[2], sync_grandchild_pipe[2];

  /*
   * Setup a pipe to send logs to the parent. This should happen
   * first, because bail will use that pipe.
   */
  setup_logpipe();

  // sync_child_pipe[0] and sync_child_pipe[1] are now connected to each other
  // and can be used to send and receive data using the read and write functions
  if (setresgid(0, 0, 0) < 0) bail("failed to become root in user namespace");

  pipenum = getenv_int("_LIBCONTAINER_INITPIPE");
  if (pipenum < 0) {
    return;
  }

  if (write(pipenum, "", 1) != 1)
    bail("could not inform the parent we are past initial setup");

  // (To Do) Parse a config which describes setting for creating user-specific
  // container.
  nl_parse(pipenum, &config);

  if (config.namespaces) {
    write_log(DEBUG, "set process as non-dumpable");
    if (prctl(PR_SET_DUMPABLE, 0, 0, 0, 0) < 0)
      bail("failed to set process as non-dumpable");
  }

  // Create socket pair between parent and child.
  if (socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_child_pipe) < 0)
    bail("failed to setup sync pipe between parent and child");

  // Create socket pair between parent and grandchild.
  if (socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_grandchild_pipe) < 0)
    bail("failed to setup sync pipe between parent and grandchild");

  // printf("uid = %u, euid = %u, gid = %u, egid = %u\n", getuid(), geteuid(),
  // getgid(), getegid());
  current_stage = setjmp(env);
  switch (current_stage) {
    // The runc init parent process creates new child process, the uid map, and
    // gid map. The child process creates a grandchild process and sends PID.
    case STAGE_PARENT: {
      pid_t stage1_pid = -1, stage2_pid = -1;
      bool stage1_complete, stage2_complete;

      prctl(PR_SET_NAME, (unsigned long)"runc:[0:PARENT]", 0, 0, 0);

      write_log(DEBUG, "stage-1");
      stage1_pid = clone_parent(&env, STAGE_CHILD);
      if (stage1_pid < 0) bail("unable to spawn stage-1");
      syncfd = sync_child_pipe[1];
      if (close(sync_child_pipe[0]) < 0)
        bail("failed to close sync_child_pipe[0] fd");
      stage1_complete = false;
      write_log(DEBUG, "stage-1 synchronisation loop");
      while (!stage1_complete) {
        enum sync_t s;
        if (read(syncfd, &s, sizeof(s)) != sizeof(s)) {
          bail("failed to sync with stage-1: next state\n");
        }
        switch (s) {
          case SYNC_USERMAP_PLS:
            write_log(DEBUG, "stage-1 requested userns mappings");
            // printf("pid:%d, ppid:%d, stage1_pid:%d\n", getpid(), getppid(),
            // stage1_pid);
            if (update_uidmap(stage1_pid, map, strlen(map)) < 0)
              bail("failed to update uidmap");
            if (update_gidmap(stage1_pid, map, strlen(map)) < 0)
              bail("failed to update gidmap");
            s = SYNC_USERMAP_ACK;
            if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
              sane_kill(stage1_pid, SIGKILL);
              sane_kill(stage2_pid, SIGKILL);
              bail("failed to sync with stage-1: write(SYNC_USERMAP_ACK)");
            }
            break;
          case SYNC_RECVPID_PLS:
            write_log(DEBUG, "stage-1 requested pid to be forwarded");
            if (read(syncfd, &stage2_pid, sizeof(stage2_pid)) !=
                sizeof(stage2_pid)) {
              sane_kill(stage1_pid, SIGKILL);
              bail("failed to sync with stage-1: read(stage2_pid)");
            }
            s = SYNC_RECVPID_ACK;
            if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
              sane_kill(stage1_pid, SIGKILL);
              sane_kill(stage2_pid, SIGKILL);
              bail("failed to sync with stage-1: write(SYNC_RECVPID_ACK)");
            }
            write_log(DEBUG,
                      "forward stage-1 (%d) and stage-2 (%d) pids to runc",
                      stage1_pid, stage2_pid);
            int len =
                dprintf(pipenum, "{\"stage1_pid\":%d,\"stage2_pid\":%d}\n",
                        stage1_pid, stage2_pid);
            if (len < 0) {
              sane_kill(stage1_pid, SIGKILL);
              sane_kill(stage2_pid, SIGKILL);
              bail("failed to sync with runc: write(pid-JSON)");
            }
            break;
          case SYNC_CHILD_FINISH:
            write_log(DEBUG, "stage-1 complete");
            stage1_complete = true;
            break;
          default: {
            break;
          }
        }
      }
      write_log(DEBUG, "<- stage-1 synchronisation loop");
      /* Now sync with grandchild. */
      syncfd = sync_grandchild_pipe[1];
      if (close(sync_grandchild_pipe[0]) < 0)
        bail("failed to close sync_grandchild_pipe[0] fd");

      write_log(DEBUG, "-> stage-2 synchronisation loop");
      stage2_complete = false;
      while (!stage2_complete) {
        enum sync_t s;

        write_log(DEBUG, "signalling stage-2 to run");
        s = SYNC_GRANDCHILD;
        if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
          sane_kill(stage2_pid, SIGKILL);
          bail("failed to sync with child: write(SYNC_GRANDCHILD)");
        }

        if (read(syncfd, &s, sizeof(s)) != sizeof(s))
          bail("failed to sync with child: next state");

        switch (s) {
          case SYNC_CHILD_FINISH:
            write_log(DEBUG, "stage-2 complete");
            stage2_complete = true;
            break;
          default:
            bail("unexpected sync value: %u", s);
        }
      }
      write_log(DEBUG, "<- stage-2 synchronisation loop");
      write_log(DEBUG, "<~ nsexec stage-0");
      exit(0);
    }
    case STAGE_CHILD: {
      pid_t stage2_pid = -1;
      enum sync_t s;

      /* We're in a child and thus need to tell the parent if we die. */
      syncfd = sync_child_pipe[0];
      if (close(sync_child_pipe[1]) < 0)
        bail("failed to close sync_child_pipe[1] fd");

      prctl(PR_SET_NAME, (unsigned long)"runc:[1:CHILD]", 0, 0, 0);
      write_log(DEBUG, "~> nsexec stage-1");
      if (config.namespaces) join_namespaces(config.namespaces);

      if (config.cloneflags & CLONE_NEWUSER) {
        // Create new user namespace.
        if (unshare(CLONE_NEWUSER) < 0)
          bail("failed to unshare user namespace");
        s = SYNC_USERMAP_PLS;
        if (write(syncfd, &s, sizeof(s)) < 0) {
          bail("failed to sync with parent: write(SYNC_USERMAP_PLS)\n");
        }
        /* ... wait for mapping ... */
        write_log(DEBUG, "request stage-0 to map user namespace");
        if (read(syncfd, &s, sizeof(s)) != sizeof(s))
          bail("failed to sync with parent: read(SYNC_USERMAP_ACK)");
        if (s != SYNC_USERMAP_ACK)
          bail("failed to sync with parent: SYNC_USERMAP_ACK: got %u", s);

        /* Become root in the namespace proper. */
        if (setresuid(0, 0, 0) < 0)
          bail("failed to become root in user namespace");
        if (setresgid(0, 0, 0) < 0)
          bail("failed to become root in user namespace");
      }
      write_log(DEBUG, "unshare remaining namespace (except cgroupns)");
      // printf("uid = %u, euid = %u, gid = %u, egid = %u\n", getuid(),
      // geteuid(), getgid(), getegid());
      /*
       * Unshare all of the namespaces. Now, it should be noted that this
       * ordering might break in the future (especially with rootless
       * containers). But for now, it's not possible to split this into
       * CLONE_NEWUSER + [the rest] because of some RHEL SELinux issues.
       *
       * Note that we don't merge this with clone() because there were
       * some old kernel versions where clone(CLONE_PARENT | CLONE_NEWPID)
       * was broken, so we'll just do it the long way anyway.
       */
      try_unshare(config.cloneflags & ~CLONE_NEWCGROUP,
                  "remaining namespaces (except cgroupns)");
      write_log(DEBUG, "stage-2");
      stage2_pid = clone_parent(&env, STAGE_INIT);
      if (stage2_pid < 0) bail("unable to spawn stage-2");
      // printf("uid = %u, euid = %u, gid = %u, egid = %u\n", getuid(),
      // geteuid(), getgid(), getegid());

      write_log(DEBUG, "request stage-0 to forward stage-2 pid (%d)",
                stage2_pid);
      s = SYNC_RECVPID_PLS;
      if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
        sane_kill(stage2_pid, SIGKILL);
        bail("failed to sync with parent: write(SYNC_RECVPID_PLS)");
      }
      if (write(syncfd, &stage2_pid, sizeof(stage2_pid)) !=
          sizeof(stage2_pid)) {
        sane_kill(stage2_pid, SIGKILL);
        bail("failed to sync with parent: write(stage2_pid)");
      }

      /* ... wait for parent to get the pid ... */
      if (read(syncfd, &s, sizeof(s)) != sizeof(s)) {
        sane_kill(stage2_pid, SIGKILL);
        bail("failed to sync with parent: read(SYNC_RECVPID_ACK)");
      }
      if (s != SYNC_RECVPID_ACK) {
        sane_kill(stage2_pid, SIGKILL);
        bail("failed to sync with parent: SYNC_RECVPID_ACK: got %u", s);
      }
      write_log(DEBUG, "signal completion to stage-0");
      s = SYNC_CHILD_FINISH;
      if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
        sane_kill(stage2_pid, SIGKILL);
        bail("failed to sync with parent: write(SYNC_CHILD_FINISH)");
      }
      write_log(DEBUG, "<~ nsexec stage-1");
      exit(0);
    } break;
    case STAGE_INIT: {
      enum sync_t s;
      write_log(DEBUG, "STAGE_INIT");
      /* We're in a child and thus need to tell the parent if we die. */
      syncfd = sync_grandchild_pipe[0];
      if (close(sync_grandchild_pipe[1]) < 0)
        bail("failed to close sync_grandchild_pipe[1] fd");

      if (close(sync_child_pipe[0]) < 0)
        bail("failed to close sync_child_pipe[0] fd");

      prctl(PR_SET_NAME, (unsigned long)"runc:[2:INIT]", 0, 0, 0);
      write_log(DEBUG, "~> nsexec stage-2");

      if (read(syncfd, &s, sizeof(s)) != sizeof(s))
        bail("failed to sync with parent: read(SYNC_GRANDCHILD)");
      if (s != SYNC_GRANDCHILD)
        bail("failed to sync with parent: SYNC_GRANDCHILD: got %u", s);

      if (config.cloneflags & CLONE_NEWCGROUP) {
        if (unshare(CLONE_NEWCGROUP) < 0)
          bail("failed to unshare cgroup namespace");
      }

      write_log(DEBUG, "signal completion to stage-0");
      s = SYNC_CHILD_FINISH;
      if (write(syncfd, &s, sizeof(s)) != sizeof(s))
        bail("failed to sync with parent: write(SYNC_CHILD_FINISH)");

      /* Close sync pipes. */
      if (close(sync_grandchild_pipe[0]) < 0)
        bail("failed to close sync_grandchild_pipe[0] fd");

      /* Finish executing, let the Go runtime take over. */
      write_log(DEBUG, "<= nsexec container setup");
      write_log(DEBUG, "booting up go runtime ...");
      return;
    }
    default:
      bail("unknown stage '%d' for jump value", current_stage);
  }
  /* Should never be reached. */
  bail("should never be reached");
}

// int main() {
//   nsexec();
// }