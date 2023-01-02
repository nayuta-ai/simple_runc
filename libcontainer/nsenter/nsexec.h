#ifndef NEXEC_H
#define NEXEC_H

#define _GNU_SOURCE
#include <assert.h>
#include <sched.h>
#include <setjmp.h>
#include <stdlib.h>
#include <unistd.h>

struct clone_t {
  jmp_buf *env;
  int jmpval;
  char stack_ptr[1024];  // example stack size
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

#endif  // NEXEC_H