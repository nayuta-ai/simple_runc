#define _GNU_SOURCE
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stdbool.h>
#include <sched.h>
#include <sys/socket.h>
#include <sys/prctl.h>

#include "nsexec.h"
#include "namespace.h"

uint32_t readint32(uint32_t value) {
  return value;
}

// nsenter.go call nsexec function for creating containers.
void nsexec(void) {
  int pipenum;
  jmp_buf env;
  struct nlconfig_t config = { 0 };
  char map[] = "0 100000 100000\n";
  int sync_child_pipe[2], sync_grandchild_pipe[2];
  printf("start nsexec\n");
  // sync_child_pipe[0] and sync_child_pipe[1] are now connected to each other
  // and can be used to send and receive data using the read and write functions
  if (setresgid(0, 0, 0) < 0)
		bail("failed to become root in user namespace");

  pipenum = getenv_int("_LIBCONTAINER_INITPIPE");
  printf("%d\n", pipenum);
  
  // (To Do) Parse a config which describes setting for creating user-specific container.

  // Create socket pair between parent and child.
  if (socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_child_pipe) < 0)
  bail("failed to setup sync pipe between parent and child");

  // Create socket pair between parent and grandchild.
  if (socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_grandchild_pipe) < 0)
  bail("failed to setup sync pipe between parent and grandchild");

  config.cloneflags = readint32(CLONE_NEWUSER); 
  // printf("uid = %u, euid = %u, gid = %u, egid = %u\n", getuid(), geteuid(), getgid(), getegid());
  current_stage = setjmp(env);
  switch(current_stage){
    // The runc init parent process creates new child process, the uid map, and gid map.
    // The child process creates a grandchild process and sends PID.
    case STAGE_PARENT:{
      pid_t stage1_pid = -1, stage2_pid = -1;
      bool stage1_complete, stage2_complete;

      write_log(LOG_LEVEL_DEBUG, "stage-1");
      stage1_pid = clone_parent(&env, STAGE_CHILD);
      if (stage1_pid < 0) bail("unable to spawn stage-1");
      syncfd = sync_child_pipe[1];
      if (close(sync_child_pipe[0]) < 0)
      bail("failed to close sync_child_pipe[0] fd");
      stage1_complete = false;
      write_log(LOG_LEVEL_DEBUG, "stage-1 synchronisation loop");
      while (!stage1_complete) {
        enum sync_t s;
        if (read(syncfd, &s, sizeof(s)) != sizeof(s)){
          bail("failed to sync with stage-1: next state\n");
        }
        switch (s) {
          case SYNC_USERMAP_PLS:
            write_log(LOG_LEVEL_DEBUG, "stage-1 requested userns mappings");
            // printf("pid:%d, ppid:%d, stage1_pid:%d\n", getpid(), getppid(), stage1_pid);
            if (update_uidmap(stage1_pid, map, strlen(map)) < 0) bail("failed to update uidmap");
            if (update_gidmap(stage1_pid, map, strlen(map)) < 0) bail("failed to update gidmap");
            s = SYNC_USERMAP_ACK;
            if (write(syncfd, &s, sizeof(s)) != sizeof(s)){
              bail("failed to sync with stage-1: write(SYNC_USERMAP_ACK)");
            }
            break;
          case SYNC_RECVPID_PLS:
            write_log(LOG_LEVEL_DEBUG, "stage-1 requested pid to be forwarded");
            if (read(syncfd, &stage2_pid, sizeof(stage2_pid)) != sizeof(stage2_pid)) bail("failed to sync with stage-1: read(stage2_pid)");
            s = SYNC_RECVPID_ACK;
            if (write(syncfd, &s, sizeof(s)) != sizeof(s)) bail("failed to sync with stage-1: write(SYNC_RECVPID_ACK)");
            // int len = dprintf(pipenum, "{\"stage1_pid\":%d,\"stage2_pid\":%d}\n", stage1_pid,stage2_pid);
            // if (len < 0) bail("failed to sync with runc: write(pid-JSON)");
            break;
          case SYNC_CHILD_FINISH:
            write_log(LOG_LEVEL_DEBUG, "stage-1 complete");
            stage1_complete = true;
            break;
          default:{
            break;
          }
        }
      }
      write_log(LOG_LEVEL_DEBUG, "<- stage-1 synchronisation loop");
      /* Now sync with grandchild. */
			syncfd = sync_grandchild_pipe[1];
			if (close(sync_grandchild_pipe[0]) < 0)
				bail("failed to close sync_grandchild_pipe[0] fd");

			write_log(LOG_LEVEL_DEBUG, "-> stage-2 synchronisation loop");
			stage2_complete = false;
			while (!stage2_complete) {
				enum sync_t s;

				write_log(LOG_LEVEL_DEBUG, "signalling stage-2 to run");
				s = SYNC_GRANDCHILD;
				if (write(syncfd, &s, sizeof(s)) != sizeof(s)) {
					bail("failed to sync with child: write(SYNC_GRANDCHILD)");
				}

				if (read(syncfd, &s, sizeof(s)) != sizeof(s))
					bail("failed to sync with child: next state");

				switch (s) {
				case SYNC_CHILD_FINISH:
					write_log(LOG_LEVEL_DEBUG, "stage-2 complete");
					stage2_complete = true;
					break;
				default:
					bail("unexpected sync value: %u", s);
				}
			}
			write_log(LOG_LEVEL_DEBUG, "<- stage-2 synchronisation loop");
			write_log(LOG_LEVEL_DEBUG, "<~ nsexec stage-0");
			exit(0);
    }
    case STAGE_CHILD:{
      char message[1024];
      pid_t stage2_pid = -1;
			enum sync_t s;

      /* We're in a child and thus need to tell the parent if we die. */
      syncfd = sync_child_pipe[0];
      if (close(sync_child_pipe[1]) < 0)
        bail("failed to close sync_child_pipe[1] fd");
      prctl(PR_SET_NAME, (unsigned long)"runc:[1:CHILD]", 0, 0, 0);
      write_log(LOG_LEVEL_DEBUG, "~> nsexec stage-1");

      if (config.cloneflags & CLONE_NEWUSER) {
        // Create new user namespace.
        if (unshare(CLONE_NEWUSER) < 0)
          bail("failed to unshare user namespace");
        s = SYNC_USERMAP_PLS;
        if (write(syncfd, &s, sizeof(s)) < 0){
          bail("failed to sync with parent: write(SYNC_USERMAP_PLS)\n");
        }
        /* ... wait for mapping ... */
        write_log(LOG_LEVEL_DEBUG, "request stage-0 to map user namespace");
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
      write_log(LOG_LEVEL_DEBUG, "unshare remaining namespace (except cgroupns)");
      // printf("uid = %u, euid = %u, gid = %u, egid = %u\n", getuid(), geteuid(), getgid(), getegid());
      if (unshare(config.cloneflags & ~CLONE_NEWCGROUP) < 0)
				bail("failed to unshare remaining namespaces (except cgroupns)");
      write_log(LOG_LEVEL_DEBUG, "stage-2");
      stage2_pid = clone_parent(&env, STAGE_INIT);
      if (stage2_pid < 0) bail("unable to spawn stage-2");
      // printf("uid = %u, euid = %u, gid = %u, egid = %u\n", getuid(), geteuid(), getgid(), getegid());
      snprintf(message, 1024, "request stage-0 to forward stage-2 pid (%d)", stage2_pid);

      write_log(LOG_LEVEL_DEBUG, message);
      s = SYNC_RECVPID_PLS;
      if (write(syncfd, &s, sizeof(s))!=sizeof(s)) bail("failed to sync with parent: write(SYNC_RECVPID_PLS)");
      if (write(syncfd, &stage2_pid, sizeof(stage2_pid))!=sizeof(stage2_pid)) bail("failed to sync with parent: write(stage2_pid)");

      /* ... wait for parent to get the pid ... */
      if (read(syncfd, &s, sizeof(s)) != sizeof(s)) bail("failed to sync with parent: read(SYNC_RECVPID_ACK)");
      if (s != SYNC_RECVPID_ACK) bail("failed to sync with parent: SYNC_RECVPID_ACK: got %u", s);
      write_log(LOG_LEVEL_DEBUG, "signal completion to stage-0");
      s = SYNC_CHILD_FINISH;
      if (write(syncfd, &s, sizeof(s)) != sizeof(s)) bail("failed to sync with parent: write(SYNC_CHILD_FINISH)");
      write_log(LOG_LEVEL_DEBUG, "<~ nsexec stage-1");
      exit(0);
    }
    break;
    case STAGE_INIT:{
      enum sync_t s;
      write_log(LOG_LEVEL_DEBUG, "STAGE_INIT");
      /* We're in a child and thus need to tell the parent if we die. */
			syncfd = sync_grandchild_pipe[0];
			if (close(sync_grandchild_pipe[1]) < 0)
				bail("failed to close sync_grandchild_pipe[1] fd");

      if (close(sync_child_pipe[0]) < 0)
				bail("failed to close sync_child_pipe[0] fd");
      
      write_log(LOG_LEVEL_DEBUG, "~> nsexec stage-2");

			if (read(syncfd, &s, sizeof(s)) != sizeof(s))
				bail("failed to sync with parent: read(SYNC_GRANDCHILD)");
			if (s != SYNC_GRANDCHILD)
				bail("failed to sync with parent: SYNC_GRANDCHILD: got %u", s);
      
      if (config.cloneflags & CLONE_NEWCGROUP) {
				if (unshare(CLONE_NEWCGROUP) < 0)
					bail("failed to unshare cgroup namespace");
			}

      write_log(LOG_LEVEL_DEBUG, "signal completion to stage-0");
			s = SYNC_CHILD_FINISH;
			if (write(syncfd, &s, sizeof(s)) != sizeof(s))
				bail("failed to sync with parent: write(SYNC_CHILD_FINISH)");

			/* Close sync pipes. */
			if (close(sync_grandchild_pipe[0]) < 0)
				bail("failed to close sync_grandchild_pipe[0] fd");
      
      /* Finish executing, let the Go runtime take over. */
			write_log(LOG_LEVEL_DEBUG, "<= nsexec container setup");
			write_log(LOG_LEVEL_DEBUG, "booting up go runtime ...");
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