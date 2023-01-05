#define _GNU_SOURCE
#include <fcntl.h>
#include <sched.h>
#include <unistd.h>
#include <stdlib.h>
#include <stdio.h>

#define errExit(msg)    do { perror(msg); exit(EXIT_FAILURE); \
                        } while (0)

int main(int argc, char *argv[]) {
    // Open a mnt namespace of the current process.
    int fd = open(argv[1], O_RDONLY);
    if (fd < 0) {
        perror("open");
        return 1;
    }
    // Re-associate the calling thread with its namespace to the calling thread.
    if (setns(fd, 0) < 0) {
        perror("setns");
        return 1;
    }
    printf("Successfully joined namespace!\n");

    execvp(argv[2], &argv[2]);
    errExit("execvp");
}