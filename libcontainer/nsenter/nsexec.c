#define _GNU_SOURCE
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include <stdbool.h>
#include <sched.h>
#include <sys/socket.h>
#include "nsexec.h"

uint32_t readint32(uint32_t value) {
  return value;
}

// nsenter.go call nsexec function for creating containers.
void nsexec(void) {
  jmp_buf env;
  struct nlconfig_t config = { 0 };
  char map[] = "0 100000 100000\n";
  int sync_child_pipe[2], sync_grandchild_pipe[2];
  // sync_child_pipe[0] and sync_child_pipe[1] are now connected to each other
  // and can be used to send and receive data using the read and write functions

  // Create socket pair between parent and child.
  if (socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_child_pipe) < 0)
  bail("failed to setup sync pipe between parent and child");

  // Create socket pair between parent and grandchild.
  if (socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_grandchild_pipe) < 0)
  bail("failed to setup sync pipe between parent and grandchild");

  config.cloneflags = readint32(CLONE_NEWUSER);
  current_stage = setjmp(env);
  switch(current_stage){
    // The runc init parent process creates new child process, the uid map, and gid map.
    // The child process creates a grandchild process and sends PID.
    case STAGE_PARENT:{
      pid_t stage1_pid = -1;
      bool stage1_complete;

      log_message(LOG_LEVEL_DEBUG, "stage-1");
      stage1_pid = clone_parent(&env, STAGE_CHILD);
      syncfd = sync_child_pipe[1];
      if (close(sync_child_pipe[0]) < 0)
      bail("failed to close sync_child_pipe[0] fd");
      stage1_complete = false;
      log_message(LOG_LEVEL_DEBUG, "stage-1 synchronisation loop");
      while (!stage1_complete) {
        enum sync_t s;
        if (read(syncfd, &s, sizeof(s)) != sizeof(s)){
          bail("failed to sync with stage-1: next state\n");
        }
        switch (s) {
          case SYNC_USERMAP_PLS:
            log_message(LOG_LEVEL_DEBUG, "stage-1 requested userns mappings");
            int result = update_uidmap(stage1_pid, map, strlen(map));
            if (result < 0) bail("failed to update uidmap");
            // update_gidmap();
            break;
          case SYNC_RECVPID_PLS:
          case SYNC_MOUNTSOURCES_PLS:
          case SYNC_CHILD_FINISH:
            log_message(LOG_LEVEL_DEBUG, "stage-1 complete");
            stage1_complete = true;
            break;
          default:{
            // stage1_complete = true;
            break;
          }
        }
      }
    }
    case STAGE_CHILD:{
      pid_t stage2_pid = -1;
			enum sync_t s;

      if (config.cloneflags & CLONE_NEWUSER) {
        /* We're in a child and thus need to tell the parent if we die. */
        syncfd = sync_child_pipe[0];
        if (close(sync_child_pipe[1]) < 0)
          bail("failed to close sync_child_pipe[1] fd");
        // Create new user namespace.
        if (unshare(CLONE_NEWUSER) < 0)
          bail("failed to unshare user namespace");
        s = SYNC_USERMAP_PLS;
        if (write(syncfd, &s, sizeof(s)) < 0){
          bail("failed to sync with parent: write(SYNC_USERMAP_PLS)\n");
        }
        if (read(syncfd, &s, sizeof(s)) != sizeof(s))
          bail("failed to sync with parent: read(SYNC_USERMAP_ACK)");
        if (s != SYNC_USERMAP_ACK)
          bail("failed to sync with parent: SYNC_USERMAP_ACK: got %u", s);
      }
    }
    default:
      break;
  }
}

int main() {
  nsexec();
}