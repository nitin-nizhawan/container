package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"io"
	"io/ioutil"
	"archive/tar"
	"strings"
	"encoding/json"
)

func main() {
	switch os.Args[1] {
	case "run":
		parent()
	case "child":
		child()
	default:
		panic("wat should I do")
	}
}

func parent() {

	fmt.Println(os.Args[0])
	fmt.Println(os.Args[1])
	fmt.Println(os.Args[2])
	cmd := exec.Command(os.Args[0], append([]string{"child"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS,
UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Println("ERROR", err)
		os.Exit(1)
	}
}

// Untar takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
func Untar(dst string, r io.Reader) error {

/*	gzr, err := gzip.NewReader(r)
	defer gzr.Close()
	if err != nil {
		return err
	}*/

	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := dst+"/"+header.Name;//filepath.Join(dst, header.Name)

			fmt.Println("processing "+target)
		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		case tar.TypeSymlink:
			fmt.Println("Name : "+target);
			fmt.Println("LinkName : "+header.Linkname);
			os.Symlink(header.Linkname,target)
		}

	}
}


func untarDockerImage(tarfile string,dest string){
	fmt.Println("docker tar :"+tarfile)
	t1,_ := ioutil.TempDir("","dock");
	fmt.Println("untaring docker image to :"+t1)
	tarfilereader, _  := os.Open(tarfile);
	defer tarfilereader.Close()

        Untar(t1,tarfilereader) 
	raw, _ := ioutil.ReadFile(t1+"/manifest.json")
	var c []interface{}
	json.Unmarshal(raw,&c)

	obj :=  c[0].(map[string]interface{})
	layerIds := obj["Layers"].([]interface{})
        
	fmt.Println(layerIds)
	for i := range layerIds {
		filep := t1+"/"+layerIds[i].(string)
		fmt.Println(" Extracting "+filep)
		t, _ :=os.Open(filep)
		defer t.Close()
	  Untar(dest,t)
	}

	
}
func child() {
	tempDir,_ := ioutil.TempDir("","dock");
	fmt.Println("TempDir :"+tempDir);
	if strings.Index(os.Args[2],".tar") > -1 {
            // untar(os.Args[2],tempDir);
	    untarDockerImage(os.Args[2],tempDir)
	must(syscall.Mount(tempDir, tempDir, "", syscall.MS_BIND | syscall.MS_PRIVATE, ""))
	} else {
 	  syscall.Mount(os.Args[2],tempDir,"",syscall.MS_BIND | syscall.MS_PRIVATE,"");
        }
	must(os.MkdirAll(tempDir+"/proc", 0700))
	must(syscall.Mount("proc",tempDir+"/proc","proc",0,""));
	must(os.MkdirAll(tempDir+"/oldrootfs", 0700))
	must(syscall.PivotRoot(tempDir, tempDir+"/oldrootfs"))
	must(os.Chdir("/"))
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
