/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// Package grpclog defines logging for grpc.
//
// All logs in transport and grpclb packages only go to verbose level 2.
// All logs in other packages in grpc are logged in spite of the verbosity level.
//
// In the default logger,
// severity level can be set by environment variable GRPC_GO_LOG_SEVERITY_LEVEL,
// verbosity level can be set by GRPC_GO_LOG_VERBOSITY_LEVEL.
package grpclog // import "google.golang.org/grpc/grpclog"

<<<<<<< HEAD
import (
	"os"

	"google.golang.org/grpc/internal/grpclog"
)

func init() {
	SetLoggerV2(newLoggerV2())
}

// V reports whether verbosity level l is at least the requested verbose level.
func V(l int) bool {
	return grpclog.Logger.V(l)
=======
import "os"

var logger = newLoggerV2()

// V reports whether verbosity level l is at least the requested verbose level.
func V(l int) bool {
	return logger.V(l)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Info logs to the INFO log.
func Info(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Info(args...)
=======
	logger.Info(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Infof logs to the INFO log. Arguments are handled in the manner of fmt.Printf.
func Infof(format string, args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Infof(format, args...)
=======
	logger.Infof(format, args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Infoln logs to the INFO log. Arguments are handled in the manner of fmt.Println.
func Infoln(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Infoln(args...)
=======
	logger.Infoln(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Warning logs to the WARNING log.
func Warning(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Warning(args...)
=======
	logger.Warning(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Warningf logs to the WARNING log. Arguments are handled in the manner of fmt.Printf.
func Warningf(format string, args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Warningf(format, args...)
=======
	logger.Warningf(format, args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Warningln logs to the WARNING log. Arguments are handled in the manner of fmt.Println.
func Warningln(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Warningln(args...)
=======
	logger.Warningln(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Error logs to the ERROR log.
func Error(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Error(args...)
=======
	logger.Error(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Errorf logs to the ERROR log. Arguments are handled in the manner of fmt.Printf.
func Errorf(format string, args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Errorf(format, args...)
=======
	logger.Errorf(format, args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Errorln logs to the ERROR log. Arguments are handled in the manner of fmt.Println.
func Errorln(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Errorln(args...)
=======
	logger.Errorln(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Fatal logs to the FATAL log. Arguments are handled in the manner of fmt.Print.
// It calls os.Exit() with exit code 1.
func Fatal(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Fatal(args...)
=======
	logger.Fatal(args...)
>>>>>>> cbc9bb05... fixup add vendor back
	// Make sure fatal logs will exit.
	os.Exit(1)
}

// Fatalf logs to the FATAL log. Arguments are handled in the manner of fmt.Printf.
<<<<<<< HEAD
// It calls os.Exit() with exit code 1.
func Fatalf(format string, args ...interface{}) {
	grpclog.Logger.Fatalf(format, args...)
=======
// It calles os.Exit() with exit code 1.
func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
>>>>>>> cbc9bb05... fixup add vendor back
	// Make sure fatal logs will exit.
	os.Exit(1)
}

// Fatalln logs to the FATAL log. Arguments are handled in the manner of fmt.Println.
// It calle os.Exit()) with exit code 1.
func Fatalln(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Fatalln(args...)
=======
	logger.Fatalln(args...)
>>>>>>> cbc9bb05... fixup add vendor back
	// Make sure fatal logs will exit.
	os.Exit(1)
}

// Print prints to the logger. Arguments are handled in the manner of fmt.Print.
//
// Deprecated: use Info.
func Print(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Info(args...)
=======
	logger.Info(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Printf prints to the logger. Arguments are handled in the manner of fmt.Printf.
//
// Deprecated: use Infof.
func Printf(format string, args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Infof(format, args...)
=======
	logger.Infof(format, args...)
>>>>>>> cbc9bb05... fixup add vendor back
}

// Println prints to the logger. Arguments are handled in the manner of fmt.Println.
//
// Deprecated: use Infoln.
func Println(args ...interface{}) {
<<<<<<< HEAD
	grpclog.Logger.Infoln(args...)
=======
	logger.Infoln(args...)
>>>>>>> cbc9bb05... fixup add vendor back
}
