package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"io/ioutil"
)

func main() {
	switch os.Args[1] {
	case "run":
		parent()
	case "init":
		child()
	default:
		panic("wat should I do")
	}
}

func parent() {

	fmt.Println(os.Args[0])
	fmt.Println(os.Args[1])
	fmt.Println(os.Args[2])
	cmd := exec.Command(os.Args[0], append([]string{"init"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
}

func child() {
	must(syscall.Mount("", "/", "", syscall.MS_REC | syscall.MS_PRIVATE, ""))
	tempDir,_ := ioutil.TempDir("","dock");
	fmt.Println("TempDir :"+tempDir);
	syscall.Mount(os.Args[2],tempDir,"",syscall.MS_BIND | syscall.MS_PRIVATE,"");
	must(os.MkdirAll(tempDir+"/oldrootfs", 0700))
	must(syscall.PivotRoot(tempDir, tempDir+"/oldrootfs"))
	must(os.Chdir("/"))
	must(os.MkdirAll("/proc", 0700))
	must(syscall.Mount("proc","/proc","proc",0,""));
        syscall.Unmount("oldrootfs",syscall.MNT_DETACH)
	os.RemoveAll("oldrootfs")
	cmd := exec.Command(os.Args[3], os.Args[4:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
