/*
Usage:
go run art-app.go [privKey] [miner ip:port]
*/

package main

import "./blockartlib"

import (
	"fmt"
	"os"
	"crypto/x509"
	"encoding/hex"
  "time"
)

type App struct {
	canvas   blockartlib.Canvas
	settings blockartlib.CanvasSettings
	shapes   map[string]string
	blocks   map[string]string
}

type testLogger struct {
	prefix string
}

func main() {
  logger := NewLogger("Artnodes")
	args := os.Args[1:]
	if len(args) < 3 {
		fmt.Println("Usage: go run art-app.go [privKeyA] [minerA ip:port] [privKeyB] [minerB ip:port]")
		return
	}

	privBytesA, _ := hex.DecodeString(args[0])
	privKeyA, err := x509.ParseECPrivateKey(privBytesA)
	if checkError(err) != nil {
		return
	}

  privBytesB, _ := hex.DecodeString(args[1])
	privKeyB, err := x509.ParseECPrivateKey(privBytesB)
	if checkError(err) != nil {
		return
	}

	appA := new(App)
  appB := new(App)

	minerAddrA := args[2]
  minerAddrB := args[3]

  // Testing connection for two miners

  testCase := fmt.Sprintf("Opening canvas with two miners")
	appA.canvas, appA.settings, err = blockartlib.OpenCanvas(minerAddrA, *privKeyA)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
    return
	}

  appB.canvas, appB.settings, err = blockartlib.OpenCanvas(minerAddrB, *privKeyB)
	if checkError(err) != nil {
		logger.TestResult(testCase, false)
    return
	}
  fmt.Println("Connected to ink miner at " + minerAddrA)
  fmt.Println("Connected to ink miner at " + minerAddrB)
  logger.TestResult(testCase, true)

  // Adding two different shapes from two different miners
  fmt.Println("Go to sleep for 10 seconds to build up ink")
  time.Sleep(10 * time.Second)
  fmt.Println("Launching test cases...")
  testCase = fmt.Sprintf("Creating two different shapes from two different miners")

  blockA := make(chan string)
  blockB := make(chan string)
  shapeA := make(chan string)
  shapeB := make(chan string)
  validateNum := uint8(2)

  go func() {
    shapeHash, blockHash, _, err := appA.canvas.AddShape(uint8(validateNum), blockartlib.PATH, "M 10 20 L 0 5", "transparent", "red")
    if checkError(err) != nil {
      logger.TestResult(testCase, false)
      return
    } else {
      blockA <- blockHash
      shapeA <- shapeHash
    }
  }()

  go func() {
    shapeHash, blockHash, _, err := appB.canvas.AddShape(uint8(validateNum), blockartlib.PATH, "M 0 0 L 5 0", "transparent", "blue")
    if checkError(err) != nil {
      logger.TestResult(testCase, false)
      return
    } else {
      blockB <- blockHash
      shapeB <- shapeHash
    }
  }()

  for i := 0; i < 2; i++ {
    select {
    case bhA := <-blockA:
      fmt.Println("received", bhA)
      logger.TestResult(testCase, true)
    case bhB := <-blockB:
      fmt.Println("received", bhB)
      logger.TestResult(testCase, true)
    }
  }

  // Check if the two hashes were mined in the same block - odds are they should be
  testCase = fmt.Sprintf("Deleting a shape that you don't own")
  var shapeHashA string
  var shapeHashB string

  for i := 0; i < 2; i++ {
    select {
    case shA := <-shapeA:
      shapeHashA = shA
      fmt.Println(shapeHashA)
    case shB := <-shapeB:
      shapeHashB = shB
      fmt.Println(shapeHashB)
    }
  }

  _, err = appA.canvas.DeleteShape(validateNum, shapeHashB)
  if checkError(err) != nil {
    logger.TestResult(testCase, true)
  } else {
    logger.TestResult(testCase, false)
  }

  // Adding two shapes from two different miners that intersect/overlap
  // Not sure how to assert that arbitrarily one will throw error while other will not
  testCase = fmt.Sprintf("Adding two shapes where one intersects/overlaps the other")

  errCh := make(chan error)

  go func() {
    shapeHash, blockHash, _, err := appA.canvas.AddShape(uint8(validateNum), blockartlib.PATH, "M 30 30 L 0 5", "transparent", "red")
    if checkError(err) != nil {
      errCh <- err
    } else {
      blockA <- blockHash
      shapeA <- shapeHash
    }
  }()

  go func() {
    shapeHash, blockHash, _, err := appB.canvas.AddShape(uint8(validateNum), blockartlib.PATH, "M 30 30 L 5 0", "transparent", "blue")
    if checkError(err) != nil {
      errCh <- err
    } else {
      blockB <- blockHash
      shapeB <- shapeHash
    }
  }()

  for i := 0; i < 2; i++ {
    select {
    case blkA := <-blockA:
      fmt.Println(blkA)
    case blkB := <-blockB:
      fmt.Println(blkB)
    case errAB := <-errCh:
      fmt.Println(errAB)
      logger.TestResult(testCase, true)
    }
  }

}


func NewLogger(prefix string) testLogger {
	return testLogger{prefix: prefix}
}

func (l testLogger) log(message string) {
	fmt.Printf("[%s][%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), l.prefix, message)
}

func (l testLogger) TestResult(description string, success bool) {
	var label string
	if success {
		label = "OK"
	} else {
		label = "ERROR"
	}

	l.log(fmt.Sprintf("%-70s%-10s", description, label))
}

//
// // If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}
