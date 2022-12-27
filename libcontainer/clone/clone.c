#define _GNU_SOURCE
#include <sys/wait.h>
#include <sys/utsname.h>
#include <sched.h>
#include <string.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

#define errExit(msg)    do { perror(msg); exit(EXIT_FAILURE); \
                        } while (0)

extern char **environ;

// start function of cloned child process
static int
childFunc(void *arg)
{
    struct utsname uts;

    // Change hostname in the UTS namespace of the child process
    if (sethostname(arg, strlen(arg)) == -1)
        errExit("sethostname");

    // Retrieve and display hostname
    if (uname(&uts) == -1)
        errExit("uname");
    printf("uts.nodename in child:  %s\n", uts.nodename);

    /*
        Use sleep to leave the namespace open for a while. This allows for experimentation
        -- e.g., if another process joins this namespace, etc.
    */
    sleep(200);
    // Terminate the child process
    return 0;
}

static int
childFuncQuick(void *arg)
{
    struct utsname uts;

    // Change hostname in the UTS namespace of the child process
    if (sethostname(arg, strlen(arg)) == -1)
        errExit("sethostname");

    // Retrieve and display hostname
    if (uname(&uts) == -1)
        errExit("uname");
    printf("uts.nodename in child:  %s\n", uts.nodename);
    
    char *argv[3];
    argv[0] = "/bin/ls";
    argv[1] = "./";
    argv[2] = NULL;
    execve(argv[0], argv, environ);
    // Terminate the child process
    return 0;
}

#define STACK_SIZE (1024 * 1024) // Stack size of child processes to be cloned

int
main(int argc, char *argv[])
{
    int ch, tflag;
    char *stack;                    /* Top of stack buffer */
    char *stackTop;                 /* Tail of stack buffer */
    pid_t pid;
    struct utsname uts;

    while ((ch = getopt(argc, argv, "t")) != -1) {
		switch (ch) {
		case 't':
			tflag = 1;	/* -t */
			break;
		default:
			return 1;
		}
	}
	argc -= optind;
	argv += optind;

    if (argc < 1) {
        fprintf(stderr, "Usage: %s <child-hostname>\n", argv[0]);
        exit(EXIT_SUCCESS);
    }

    /* Allocate stacks for child processes */
    stack = malloc(STACK_SIZE);
    if (stack == NULL)
        errExit("malloc");
    stackTop = stack + STACK_SIZE;

    /* Create a child process with its own UTS namespace;
        Child process starts childFunc() execution */

    if (tflag == 1) 
        pid = clone(childFuncQuick, stackTop, CLONE_NEWUTS | SIGCHLD, argv[0]);
    else
        pid = clone(childFunc, stackTop, CLONE_NEWUTS | SIGCHLD, argv[0]);
    if (pid == -1)
        errExit("clone");
    printf("clone() returned %ld\n", (long) pid);

    /* Parent Process Execution Comes Here */

    sleep(1);           /* Give the child process time to change the hostname */

    /* Displays the hostname in the UTS namespace of the parent process;
        This is different from the hostname in the UTS namespace of the child process */

    if (uname(&uts) == -1)
        errExit("uname");
    printf("uts.nodename in parent: %s\n", uts.nodename);

    if (waitpid(pid, NULL, 0) == -1)    /* Wait for the child process */
        errExit("waitpid");
    printf("child has terminated\n");

    exit(EXIT_SUCCESS);
}