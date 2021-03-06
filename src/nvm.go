package main

import (
  "fmt"
  "os"
  "os/exec"
  "strings"
  "path/filepath"
  "net/http"
  "io"
  "io/ioutil"
  "regexp"
  "bytes"
  "encoding/json"
  "archive/zip"
  "log"
  "bufio"
)

var root = ""
var symlink = ""
var settingsFile = os.Getenv("APPDATA")+"\\nvm\\settings.txt"

func main() {
  args := os.Args
  detail := ""

  setRootDir()

  // Capture any additional arguments
  if (len(args) > 2) {
    detail = strings.ToLower(args[2])
  }

  if (len(args) < 2){
    help()
    return
  }

  // Run the appropriate method
  switch args[1] {
    case "install": install(detail)
    case "uninstall": uninstall(detail)
    case "use": use(detail)
    case "list": list(detail)
    case "on": enable()
    case "off": disable()
    case "root":
      if len(args) == 3 {
        updateRootDir(args[2])
      } else {
        fmt.Println("\nCurrent Root: "+root)
      }
    default: help()
  }
}

func install(version string) {
  if version == "" {
    fmt.Println("\nInvalid version.\n")
    help()
    return
  }

  // If user specifies "latest" version, find out what version is
  if version == "latest" {
    content := getRemoteTextFile("http://nodejs.org/dist/latest/SHASUMS.txt")
    re := regexp.MustCompile("node-v(.+)+msi")
    reg := regexp.MustCompile("node-v|-x.+")
    version = reg.ReplaceAllString(re.FindString(content),"")
  }

  // Check to see if the version is already installed
  if !isVersionInstalled(version) {

    if !isVersionAvailable(version){
      fmt.Println("Version "+version+" is not available. If you are attempting to download a \"just released\" version,")
      fmt.Println("it may not be recognized by the nvm service yet (updated hourly). If you feel this is in error and")
      fmt.Println("you know the version exists, please visit http://github.com/coreybutler/nodedistro and submit a PR.")
      return
    }

    // Make the output directories
    os.Mkdir(root+"\\v"+version,os.ModeDir)
    os.Mkdir(root+"\\v"+version+"\\node_modules",os.ModeDir)

    // Download node
    success := downloadNodeJs(version);

    // If successful, add npm
    if success {
      npmv := getNpmVersion(version)
      success = downloadNpm(getNpmVersion(version))
      if success {
        fmt.Printf("Installing npm v"+npmv+"...")

        // Extract npm to the temp directory
        unzip(os.TempDir()+"\\npm-v"+npmv+".zip",os.TempDir()+"\\nvm-npm")

        // Copy the npm and npm.cmd files to the installation directory
        os.Rename(os.TempDir()+"\\nvm-npm\\npm-"+npmv+"\\bin\\npm",root+"\\v"+version+"\\npm")
        os.Rename(os.TempDir()+"\\nvm-npm\\npm-"+npmv+"\\bin\\npm.cmd",root+"\\v"+version+"\\npm.cmd")
        os.Rename(os.TempDir()+"\\nvm-npm\\npm-"+npmv,root+"\\v"+version+"\\node_modules\\npm")

        // Remove the source file
        os.RemoveAll(os.TempDir()+"\\nvm-npm")

        fmt.Printf(" done.")
        fmt.Println("\n\nInstallation complete. If you want to use this version, type\n\nnvm use "+version)
      } else {
        fmt.Println("Could not download npm for node v"+version+".")
        fmt.Println("Please visit https://github.com/npm/npm/releases/tag/v"+npmv+" to download npm.")
        fmt.Println("It should be extracted to "+root+"\\v"+version)
      }
    } else {
      fmt.Println("Could not download node.js executable for version "+version+".")
    }

    // If this is ever shipped for Mac, it should use homebrew.
    // If this ever ships on Linux, it should be on bintray so it can use yum, apt-get, etc.

    return
   } else {
     fmt.Println("Version "+version+" is already installed.")
     return
   }

}

func uninstall(version string) {
  // Make sure a version is specified
  if len(version) == 0 {
    fmt.Println("Provide the version you want to uninstall.")
    help()
    return
  }

  // Determine if the version exists and skip if it doesn't
  if isVersionInstalled(version) {
    fmt.Printf("Uninstalling node v"+version+"...")
    e := os.RemoveAll(root+"\\v"+version)
    if e != nil {
      fmt.Println("Error removing node v"+version)
      fmt.Println("Check to assure "+root+"\\v"+version+" no longer exists.")
    }
    fmt.Printf(" done")
  } else {
    fmt.Println("node v"+version+" is not installed. Type \"nvm list\" to see what is installed.")
  }
  return
}

func use(version string) {
  // Make sure the version is installed. If not, warn.
  if !isVersionInstalled(version) {
    fmt.Println("node v"+version+" is not installed.")
    return
  }

  // Create or update the symlink
  sym, serr := os.Stat(symlink)
  serr = serr
  if sym != nil {
    cmd := exec.Command(root+"\\elevate.cmd", "cmd", "/C", "rmdir", symlink)
    var output bytes.Buffer
    var _stderr bytes.Buffer
    cmd.Stdout = &output
    cmd.Stderr = &_stderr
    perr := cmd.Run()
    if perr != nil {
        fmt.Println(fmt.Sprint(perr) + ": " + _stderr.String())
        return
    }
  }

  c := exec.Command(root+"\\elevate.cmd", "cmd", "/C", "mklink", "/D", symlink, root+"\\v"+version)
  var out bytes.Buffer
  var stderr bytes.Buffer
  c.Stdout = &out
  c.Stderr = &stderr
  err := c.Run()
  if err != nil {
      fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
      return
  }
  fmt.Println("Now using node v"+version)
}

func list(listtype string) {
//  if listtype == "" {
    listtype = "installed"
//  }
//  if listtype != "installed" && listtype != "available" {
//    fmt.Println("\nInvalid list option.\n\nPlease use on of the following\n  - nvm list\n  - nvm list installed\n  - nvm list available")
//    help()
//    return
//  }
  if listtype == "installed" {
    fmt.Println("")
    vers := ""
    cmd := exec.Command("node","-v")
    str, err := cmd.Output()
    if err == nil {
      vers = strings.Trim(string(str)," \n\r")
    }

    dir := ""
    files, _ := ioutil.ReadDir(root)
    for _, f := range files {
      if f.IsDir() {
        isnode, verr := regexp.MatchString("v",f.Name())
        verr = verr
        if isnode {
          dir = f.Name()
          if f.Name() == vers {
            fmt.Printf("  * ")
          } else {
            fmt.Printf("    ")
          }
          fmt.Printf(regexp.MustCompile("v").ReplaceAllString(f.Name(),""))
          if f.Name() == vers {
            fmt.Printf(" (In Use)")
          }
          fmt.Printf("\n")
        }
      }
    }
    if len(strings.Trim(dir," \n\r")) == 0 {
      fmt.Println("No installations recognized.")
    }
//  } else {
//    fmt.Printf("List "+listtype)
  }
}

func enable() {
  dir := ""
  files, _ := ioutil.ReadDir(root)
  for _, f := range files {
    if f.IsDir() {
      isnode, verr := regexp.MatchString("v",f.Name())
      verr = verr
      if isnode {
        dir = f.Name()
      }
    }
  }
  fmt.Println("nvm enabled")
  if dir != "" {
    use(strings.Trim(regexp.MustCompile("v").ReplaceAllString(dir,"")," \n\r"))
  } else {
    fmt.Println("No versions of node.js found. Try installing the latest by typing nvm install latest")
  }
}

func disable() {
  cmd := exec.Command(root+"\\elevate.cmd", "cmd", "/C", "rmdir", symlink)
  cmd.Run()
  fmt.Println("nvm disabled")
}

func help() {
  fmt.Println("\nUsage:\n")
  fmt.Println("  nvm install <version>        : The version can be a node.js version or \"latest\" for the latest stable version.")
  fmt.Println("  nvm uninstall <version>      : The version must be a specific version.")
  fmt.Println("  nvm use <version>            : Switch to use the specified version.")
  fmt.Println("  nvm list                     : List what is currently installed.")
  fmt.Println("  nvm on                       : Enable node.js version management.")
  fmt.Println("  nvm off                      : Disable node.js version management.")
  fmt.Println("  nvm root <path>              : Set the directory where nvm should store different versions of node.js.")
  fmt.Println("                                 If <path> is not set, the current root will be displayed.\n")
}

func getRemoteTextFile(url string) string {
  response, httperr := http.Get(url)
  if httperr != nil {
    fmt.Println("\nCould not retrieve "+url+".\n\n")
    fmt.Printf("%s", httperr)
    os.Exit(1)
  } else {
    defer response.Body.Close()
    contents, readerr := ioutil.ReadAll(response.Body)
    if readerr != nil {
      fmt.Printf("%s", readerr)
      os.Exit(1)
    }
    return string(contents)
  }
  os.Exit(1)
  return ""
}

// Given a node.js version, returns the associated npm version
func getNpmVersion(nodeversion string) string {

  // Get raw text
  text := getRemoteTextFile("https://raw.githubusercontent.com/coreybutler/nodedistro/master/nodeversions.json")

  // Parse
  var data interface{}
  json.Unmarshal([]byte(text), &data);
  body := data.(map[string]interface{})
  all := body["all"]
  npm := all.(map[string]interface{})

  return npm[nodeversion].(string)
}

func downloadNodeJs(v string) bool {

  url := "http://nodejs.org/dist/v"+v+"/node.exe"
  fileName := root+"\\v"+v+"\\node.exe"

  fmt.Printf("Downloading node.js version "+v+"... ")

  output, err := os.Create(fileName)
  if err != nil {
    fmt.Println("Error while creating", fileName, "-", err)
  }
  defer output.Close()

  response, err := http.Get(url)
  if err != nil {
    fmt.Println("Error while downloading", url, "-", err)
  }
  defer response.Body.Close()

  n, err := io.Copy(output, response.Body)
  if err != nil {
    fmt.Println("Error while downloading", url, "-", err)
  }

  if response.Status[0:3] == "200" {
    fmt.Println(n, "bytes downloaded.")
  } else {
    fmt.Println("ERROR")
  }

  return response.Status[0:3] == "200"
}

func downloadNpm(v string) bool {

  url := "https://github.com/npm/npm/archive/v"+v+".zip"
  fileName := os.TempDir()+"\\"+"npm-v"+v+".zip"

  fmt.Printf("Downloading npm version "+v+"... ")

  output, err := os.Create(fileName)
  if err != nil {
    fmt.Println("Error while creating", fileName, "-", err)
  }
  defer output.Close()

  response, err := http.Get(url)
  if err != nil {
    fmt.Println("Error while downloading", url, "-", err)
  }
  defer response.Body.Close()

  n, err := io.Copy(output, response.Body)
  if err != nil {
    fmt.Println("Error while downloading", url, "-", err)
  }

  if response.Status[0:3] == "200" {
    fmt.Println(n, "bytes downloaded.")
  } else {
    fmt.Println("ERROR")
  }

  return response.Status[0:3] == "200"
}

func isVersionInstalled(version string) bool {
  src, err := os.Stat(root+"\\v"+version)
  src = src
  return err == nil
}

// Function courtesy http://stackoverflow.com/users/1129149/swtdrgn
func unzip(src, dest string) error {
  r, err := zip.OpenReader(src)
  if err != nil {
      return err
  }
  defer r.Close()

  for _, f := range r.File {
    rc, err := f.Open()
    if err != nil {
        return err
    }
    defer rc.Close()

    fpath := filepath.Join(dest, f.Name)
    if f.FileInfo().IsDir() {
      os.MkdirAll(fpath, f.Mode())
    } else {
      var fdir string
      if lastIndex := strings.LastIndex(fpath,string(os.PathSeparator)); lastIndex > -1 {
        fdir = fpath[:lastIndex]
      }

      err = os.MkdirAll(fdir, f.Mode())
      if err != nil {
        log.Fatal(err)
        return err
      }
      f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
      if err != nil {
        return err
      }
      defer f.Close()

      _, err = io.Copy(f, rc)
      if err != nil {
        return err
      }
    }
  }
  return nil
}

func readLines(path string) ([]string, error) {
  file, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  defer file.Close()

  var lines []string
  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    lines = append(lines, scanner.Text())
  }
  return lines, scanner.Err()
}

func updateRootDir(path string) {
  ok, err := os.Stat(path)
  if err != nil {
    fmt.Println(path+" does not exist or could not be found.")
    return
  }
  ok = ok

  content := "root: "+strings.Trim(path," \n\r")+"\r\npath: "+strings.Trim(symlink," \n\r")
  ioutil.WriteFile(settingsFile, []byte(content), 0644)
  root = path
  fmt.Println("\nRoot has been set to "+path)
}

func setRootDir() {
  lines, err := readLines(settingsFile)
  if err != nil {
    fmt.Println("\nERROR",err)
    os.Exit(1)
  }

  // Process each line and extract the value
  for _, line := range lines {
    if strings.Contains(line,"root:") {
      root = strings.Trim(regexp.MustCompile("root:").ReplaceAllString(line,"")," \r\n")
    } else if strings.Contains(line,"path:") {
      symlink = strings.Trim(regexp.MustCompile("path:").ReplaceAllString(line,"")," \r\n")
    }
  }

  // Make sure the directories exist
  p, e := os.Stat(root)
  if e != nil {
    fmt.Println(root+" could not be found or does not exist. Exiting.")
    return
    p=p
  }
}

func isVersionAvailable(v string) bool {
  // Check the service to make sure the version is available
  text := getRemoteTextFile("https://raw.githubusercontent.com/coreybutler/nodedistro/master/nodeversions.json")

  // Parse
  var data interface{}
  json.Unmarshal([]byte(text), &data);
  body := data.(map[string]interface{})
  all := body["all"]
  npm := all.(map[string]interface{})

  return npm[v] != nil
}
