#include <gtest/gtest.h>
#include <setjmp.h>
#include <stdlib.h>
#include <sys/types.h>
#include <sys/wait.h>
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
    exit(0);  // exit the child process
  } else {
    // setjmp returns non-zero when longjmp is called
    // code to handle the longjmp goes here
    waitpid(pid, &status, 0);  // wait for the child to exit
    EXPECT_EQ(WEXITSTATUS(status), 0);  // check that the child exited with status 0
  }
}