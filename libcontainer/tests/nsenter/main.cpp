#include <gtest/gtest.h>
#include <setjmp.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
#include "../../nsenter/nsexec.h"

int main(int argc, char **argv) {
  ::testing::InitGoogleTest(&argc, argv);
  return RUN_ALL_TESTS();
}

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

  log_message(LOG_LEVEL_ERROR, "This is an error message");
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

  log_message(LOG_LEVEL_WARNING, "This is a warning message");
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

  log_message(LOG_LEVEL_INFO, "This is an info message");

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

  log_message(LOG_LEVEL_DEBUG, "This is a debug message");

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
