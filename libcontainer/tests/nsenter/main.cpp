#include <gtest/gtest.h>
#include <stdio.h>
#include <fcntl.h>
#include <setjmp.h>
#include <stdlib.h>
#include <sched.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <sys/socket.h>
#include <unistd.h>
#include "../../nsenter/nsexec.h"

TEST(CloneParentTest, BasicTest) {
  jmp_buf env;
  int jmpval = 10;
  int status;

  // test the clone_parent function
  pid_t pid = clone_parent(&env, jmpval);
  EXPECT_GT(pid, 0);  // check that clone returned a valid pid

  if (setjmp(env) == 0) {
    // setjmp returns 0 the first time it is called
    // code that may cause a longjmp call goes here
  } else {
    // setjmp returns non-zero when longjmp is called
    // code to handle the longjmp goes here
    waitpid(pid, &status, 0);  // wait for the child to exit
    EXPECT_EQ(WEXITSTATUS(status), 0);  // check that the child exited with status 0
  }
}



TEST(LogMessageTest, LogsErrorMessage) {
  // Redirect stdout to a string
  testing::internal::CaptureStdout();

  write_log(LOG_LEVEL_ERROR, "This is an error message");
  rewind(stdout);

  char expected_output[1024];
  time_t t = time(NULL);
  struct tm tm = *localtime(&t);
  sprintf(expected_output, "[%04d-%02d-%02d %02d:%02d:%02d] [ERROR] This is an error message\n",
          tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
          tm.tm_hour, tm.tm_min, tm.tm_sec);

  // Get the captured output
  std::string output = testing::internal::GetCapturedStdout();

  // Verify that the output is correct
  EXPECT_EQ(output, expected_output);
}

TEST(LogMessageTest, LogsWarningMessage) {
  // Redirect stdout to a string
  testing::internal::CaptureStdout();

  write_log(LOG_LEVEL_WARNING, "This is a warning message");
  rewind(stdout);

  char expected_output[1024];
  time_t t = time(NULL);
  struct tm tm = *localtime(&t);
  sprintf(expected_output, "[%04d-%02d-%02d %02d:%02d:%02d] [WARNING] This is a warning message\n",
          tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
          tm.tm_hour, tm.tm_min, tm.tm_sec);

  // Get the captured output
  std::string output = testing::internal::GetCapturedStdout();

  // Verify that the output is correct
  EXPECT_EQ(output, expected_output);
}

TEST(LogMessageTest, LogsInfoMessage) {
  // Redirect stdout to a string
  testing::internal::CaptureStdout();

  write_log(LOG_LEVEL_INFO, "This is an info message");

  char expected_output[1024];
  time_t t = time(NULL);
  struct tm tm = *localtime(&t);
  sprintf(expected_output, "[%04d-%02d-%02d %02d:%02d:%02d] [INFO] This is an info message\n",
          tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
          tm.tm_hour, tm.tm_min, tm.tm_sec);

  // Get the captured output
  std::string output = testing::internal::GetCapturedStdout();

  // Verify that the output is correct
  EXPECT_EQ(output, expected_output);
}

TEST(LogMessageTest, LogsDebugMessage) {
  // Redirect stdout to a string
  testing::internal::CaptureStdout();

  write_log(LOG_LEVEL_DEBUG, "This is a debug message");

  char expected_output[1024];
  time_t t = time(NULL);
  struct tm tm = *localtime(&t);
  sprintf(expected_output, "[%04d-%02d-%02d %02d:%02d:%02d] [DEBUG] This is a debug message\n",
          tm.tm_year + 1900, tm.tm_mon + 1, tm.tm_mday,
          tm.tm_hour, tm.tm_min, tm.tm_sec);

  // Get the captured output
  std::string output = testing::internal::GetCapturedStdout();

  // Verify that the output is correct
  EXPECT_EQ(output, expected_output);
}

TEST(UpdateUidmapTest, ValidMap) {
  jmp_buf env;
  int res, sync_child_pipe[2];
  /* Create a sample map and write it to a temporary file */
  char map[] = "         0          0 4294967295\n";
  int fd;

  /* Create a socket that connects STAGE_PARENT and STAGE_CHILD */
  res = socketpair(AF_LOCAL, SOCK_STREAM, 0, sync_child_pipe);
  EXPECT_GE(res, 0);

  /* setjmp returns STAGE_PARENT the first time it is called
   code that may cause a clone_parent function call to go STAGE_CHILD */
  current_stage = setjmp(env);
  switch(current_stage){
    case STAGE_PARENT:{
      pid_t child_pid = -1;
      enum sync_t s;
      /* Create a child process and get a child PID */
      child_pid = clone_parent(&env, STAGE_CHILD);
      EXPECT_GE(child_pid, 0);
      /* Open a pipe that communicates with CHILD_STAGE */
      syncfd = sync_child_pipe[1];
      res = close(sync_child_pipe[0]);
      EXPECT_GE(res, 0);

      /* ... wait for creating a child process ... */
      if (read(syncfd, &s, sizeof(s)) != sizeof(s)){
        bail("failed to sync with stage-1: next state\n");
      }
      switch (s) {
        case SYNC_USERMAP_PLS:
          char map_path[1024];
          char buf[1024];
          
          res = update_uidmap(child_pid, map, strlen(map));
          EXPECT_EQ(res, 0);
          /* Read uid_map and check whether it updates correctly. */
          snprintf(map_path, sizeof(map_path), "/proc/%d/uid_map", child_pid);
          fd = open(map_path, O_RDONLY);
          res = read(fd, &buf, sizeof(buf));
          EXPECT_GE(res, 0);
          EXPECT_STREQ(buf, map);

          s = SYNC_USERMAP_ACK;
          res = write(syncfd, &s, sizeof(s));
          EXPECT_EQ(res, sizeof(s));
          break;
      }
    }
    break;
    case STAGE_CHILD:{
			enum sync_t s;

      /* We're in a child and thus need to tell the parent if we die. */
      syncfd = sync_child_pipe[0];
      res = close(sync_child_pipe[1]);
      EXPECT_GE(res, 0);
      /* Create new user namespace. */
      res = unshare(CLONE_NEWUSER);
      EXPECT_GE(res, 0);
      s = SYNC_USERMAP_PLS;
      res = write(syncfd, &s, sizeof(s));
      EXPECT_EQ(res, sizeof(s));
      /* ... wait for mapping ... */
      res = read(syncfd, &s, sizeof(s));
      EXPECT_EQ(res, sizeof(s));
      EXPECT_EQ(s, SYNC_USERMAP_ACK);
      /* Finish a child process */
      exit(0);
    }
  }
}
