package srctoken

import (
  "bufio"
  "fmt"
  "io"
  "os"
  "path/filepath"
  "regexp"
)

type Token string
type Path string

type TokenizeOptions struct {
  FolderExcludeRegex, FileIncludeRegex string
}

func isSplitter(ch byte) bool {
  return !(
    (ch >= 'A' && ch <= 'Z') ||
    (ch >= 'a' && ch <= 'z') ||
    (ch >= '0' && ch <= '9') ||
    (ch == '_' || ch == '-' || ch == '.' || ch == '*' || ch == '&'))
}

var minTokenLen int = 2
func CodeWordSplitter(data []byte, atEOF bool) (int, []byte, error) {
  foundTokenChar := false
  sliceStart := 0
  splits := false

  for i, textByte := range data {
    splits = isSplitter(textByte)

    if splits && foundTokenChar {
      // We might have a token, but it might not be long enough
      if (i - sliceStart) < minTokenLen {
        // Token isn't long enough, counts as just splits.
        foundTokenChar = false
      } else {
        // Is a token!
        return i, data[sliceStart:i], nil
      }
    } else if !splits && !foundTokenChar {
      // Possibly the start of a token (if it's long enough)
      sliceStart = i
      foundTokenChar = true
    }
  }

  // EOF: check if we're ending with a valid token
  if atEOF {
    if !splits && (len(data) - sliceStart >= minTokenLen) {
      // file ended with a valid token
      return len(data), data[sliceStart:], nil
    } else {
      return len(data), nil, nil
    }
  }

  // End-of-chunk - see if we need to read more
  if splits {
    // Not in a valid token, just go to next chunk
    return len(data), nil, nil
  } else {
    // In a valid token, dump everything up to the start of the possible-token
    return sliceStart, nil, nil
  }
}

func TokenizeFile(filePath Path, reader io.Reader) {
  scanner := bufio.NewScanner(reader)
  scanner.Split(CodeWordSplitter)
  for scanner.Scan() {
    //fmt.Println(filePath, scanner.Text())
  }
}

// Scans all files in directory, returns a map of tokens -> files that contain that token
// filePath -- recursively searches this path and folders (and subfolders, etc.)
// path_include_regex -- only tokenizes files matching this regex
func TokenizeDir(parentChan chan map[Token][]Path, filePath Path, options TokenizeOptions) () {
  file, err := os.Open(string(filePath))
  // TODO(mike): handle errors better
  if err != nil {
    fmt.Printf("Open ERROR: %v\n", err)
    parentChan <- nil
    if file != nil {
      file.Close()
    }
    return
  }

  fileInfo, err := file.Stat()
  // TODO(mike): handle errors better
  if err != nil {
    fmt.Printf("Stat ERROR: %v\n", err)
    parentChan <- nil
    file.Close()
    return
  }

  if fileInfo.IsDir() {
    // Ignore folders in the regex blacklist
    if options.FolderExcludeRegex != "" {
      matched, err := regexp.MatchString(options.FolderExcludeRegex, string(filePath))
      if err != nil || matched {
        //fmt.Println("Skipping folder ", filePath)
        parentChan <- nil
        file.Close()
        return
      }
    }

    //fmt.Printf("Processing folder \"%v\"\n", filePath)

    childrenChan := make(chan map[Token][]Path)
    childFilenames, err := file.Readdirnames(0)
    file.Close()
    // TODO(mike): handle errors better
    if err != nil {
      fmt.Printf("Readdirnames ERROR: %v\n", err)
      parentChan <- nil
      return
    }

    for _, childFilename := range childFilenames {
      // Iterate through each file in directory
      childPath := Path(filepath.Join(string(filePath), childFilename))
      go TokenizeDir(childrenChan, childPath, options)
    }

    // Merge the results
    for i := 0; i < len(childFilenames); i++ {
      // TODO(mike): do token map merging
      <- childrenChan
    }
  } else {
    // It's a file, make sure in the whitelist
    if options.FileIncludeRegex != "" {
      matched, err := regexp.MatchString(options.FileIncludeRegex, string(filePath))
      if !matched || err != nil {
        //fmt.Println("Skipping file", filePath)
        if err != nil {
          fmt.Printf("Stat ERROR: %v\n", err)
        }
        parentChan <- nil
        return
      }

      //fmt.Printf("Processing file \"%v\"\n", filePath)
      TokenizeFile(filePath, file)
      file.Close()
    }
  }
  parentChan <- nil
}
