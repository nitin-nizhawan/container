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
	"time"
	"strconv"
// network setup
        "code.cloudfoundry.org/guardian/kawasaki/netns"
	"github.com/teddyking/netsetgo"
	"github.com/teddyking/netsetgo/configurer"
	"github.com/teddyking/netsetgo/device" 
	"net"
)

func main() {
	switch os.Args[1] {
	case "run":
		handle_run()
	case "launch":
		handle_launch()
	default:
		panic("wat should I do")
	}
}

func waitForNetwork() error {
	maxWait := time.Second * 3
	checkInterval := time.Second
	timeStarted := time.Now()

	for {
		interfaces, err := net.Interfaces()
		if err != nil {
			return err
		}
		//sysctl -p /etc/sysctl.conf
		//sudo sysctl -w net.ipv4.ip_forward=1
		//sudo iptables -A FORWARD -o eth0 -i brg0 -j ACCEPT
//sudo iptables -A FORWARD -i eth0 -o brg0  -j ACCEPT
		// pretty basic check ...
		// > 1 as a lo device will already exist
		if len(interfaces) > 1 {
			// configure masquerading
		        //sudo sysctl -w net.ipv4.ip_forward=1
			//sudo iptables -t nat -A POSTROUTING -s 10.10.10.0/255.255.255.0 -o eth0 -j MASQUERADE
			return nil
		}

		if time.Since(timeStarted) > maxWait {
			return fmt.Errorf("Timeout after %s waiting for network", maxWait)
		}

		time.Sleep(checkInterval)
	}
	return nil
}

func setupNetwork(pid int){

var bridgeName, bridgeAddress, containerAddress, vethNamePrefix string
bridgeName = "brg0"
bridgeAddress = "10.10.10.1/24"
vethNamePrefix = "veth"
containerAddress = "10.10.10.2/24"


	if pid == 0 {
		fmt.Println("ERROR - netsetgo needs a pid")
		os.Exit(1)
	}

	bridgeCreator := device.NewBridge()
	vethCreator := device.NewVeth()
	netnsExecer := &netns.Execer{}

	hostConfigurer := configurer.NewHostConfigurer(bridgeCreator, vethCreator)
	containerConfigurer := configurer.NewContainerConfigurer(netnsExecer)
	netset := netsetgo.New(hostConfigurer, containerConfigurer)

	bridgeIP, bridgeSubnet, err := net.ParseCIDR(bridgeAddress)
	must(err)

	containerIP, _, err := net.ParseCIDR(containerAddress)
	must(err)

	netConfig := netsetgo.NetworkConfig{
		BridgeName:     bridgeName,
		BridgeIP:       bridgeIP,
		ContainerIP:    containerIP,
		Subnet:         bridgeSubnet,
		VethNamePrefix: vethNamePrefix,
	}

	must(netset.ConfigureHost(netConfig, pid))
	must(netset.ConfigureContainer(netConfig, pid))	 
	fmt.Println("Done setting up network \n")
}
func handle_run() {

	fmt.Println(os.Args[0])
	fmt.Println(os.Args[1])
	fmt.Println(os.Args[2])
	fmt.Println("Uid : "+strconv.Itoa(os.Getuid()))
	fmt.Println("Gid : "+strconv.Itoa(os.Getgid()))
	cmd := exec.Command(os.Args[0], append([]string{"launch"}, os.Args[2:]...)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC ,
UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        75000,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        75000,
			},
		},
		GidMappingsEnableSetgroups:true,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

        if err := cmd.Start(); err!=nil {
		fmt.Println("ERRO",err)
		os.Exit(1)
	}


	setupNetwork(cmd.Process.Pid)


	if err := cmd.Wait(); err != nil {
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
			f.Close()
		case tar.TypeSymlink:
			os.Symlink(header.Linkname,target)
		}

	}
}

func readJson(file string) interface{} {
	raw, _ := ioutil.ReadFile(file)
	var c interface{}
	json.Unmarshal(raw,&c)
        return c
}
type ContainerSpec struct {
	Env []string
	Cmd []string
	Entrypoint string
}
func objArrayToStrArray(objArray []interface{}) []string{
	strArray := make([]string,len(objArray))
	for i:=0;i<len(objArray);i++ {
           strArray[i] = objArray[i].(string)
	}
	return strArray;
}
func untarDockerImage(tarfile string,dest string) *ContainerSpec{
	fmt.Println("docker tar :"+tarfile)
	t1,_ := ioutil.TempDir("","dock");
	fmt.Println("untaring docker image to :"+t1)
	tarfilereader, _  := os.Open(tarfile);
	defer tarfilereader.Close()

        Untar(t1,tarfilereader) 
	/*raw, _ := ioutil.ReadFile(t1+"/manifest.json")
	var c []interface{}
	json.Unmarshal(raw,&c)*/
        obj := readJson(t1+"/manifest.json").([]interface{})[0].(map[string]interface{})
	//obj :=  c[0].(map[string]interface{})
	layerIds := obj["Layers"].([]interface{})
	ConfigFile := obj["Config"].(string)
        configFileJson := readJson(t1+"/"+ConfigFile).(map[string]interface{})
	configObj := configFileJson["config"].(map[string]interface{})
	var entryPoint string = ""
	if configObj["Entrypoint"] != nil{
           entryPoint = configObj["Entrypoint"].([]interface{})[0].(string)
        }
	env := objArrayToStrArray(configObj["Env"].([]interface{}))
	Cmd := objArrayToStrArray(configObj["Cmd"].([]interface{}))
        spec := &ContainerSpec {
		Env:env,
		Cmd:Cmd,
		Entrypoint:entryPoint,
        }
	layerIdLen := len(layerIds)
	fmt.Println(layerIds)
	for i := range layerIds {
		filep := t1+"/"+layerIds[layerIdLen-i-1].(string)
		fmt.Println(" Extracting "+filep)
		t, _ :=os.Open(filep)
		defer t.Close()
	  Untar(dest,t)
	}

       return	spec
}
func handle_launch() {
	hostName := "root"+fmt.Sprintf("%v",time.Now().UnixNano());
	syscall.Sethostname([]byte(hostName))
	tempDir,_ := ioutil.TempDir("","dock");
	fmt.Println("TempDir :"+tempDir);
       
	spec := &ContainerSpec{
		Cmd:make([]string,0),
		Env:[]string{"PATH=/bin"},
		Entrypoint:"",
	}
	if(len(os.Args) > 3){
		spec.Cmd = os.Args[3:]
        }
	if strings.Index(os.Args[2],".tar") > -1 {
            // untar(os.Args[2],tempDir);
	    spec = untarDockerImage(os.Args[2],tempDir)
	must(syscall.Mount(tempDir, tempDir, "", syscall.MS_BIND | syscall.MS_PRIVATE, ""))
	} else {
 	  syscall.Mount(os.Args[2],tempDir,"",syscall.MS_BIND | syscall.MS_PRIVATE,"");
        }
	if(len(os.Args) > 3){
		spec.Cmd = os.Args[3:]
        }
	must(os.MkdirAll(tempDir+"/proc", 0700))
	must(syscall.Mount("proc",tempDir+"/proc","proc",0,""));
	must(os.MkdirAll(tempDir+"/oldrootfs", 0700))
	must(syscall.PivotRoot(tempDir, tempDir+"/oldrootfs"))
	must(os.Chdir("/"))
        syscall.Unmount("oldrootfs",syscall.MNT_DETACH)
	os.RemoveAll("oldrootfs")
	fmt.Println("Entrypoint :"+spec.Entrypoint)
	fmt.Println("Cmd :"+strings.Join(spec.Cmd[:],","))
	must(waitForNetwork())
        var cmd *exec.Cmd
	if spec.Entrypoint != ""{
	   cmd = exec.Command(spec.Entrypoint, spec.Cmd...)
        }   else {
		cmd = exec.Command(spec.Cmd[0], spec.Cmd[1:]...)
	}
	cmd.Env = spec.Env
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
