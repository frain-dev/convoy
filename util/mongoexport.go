package util

import (
	"fmt"
	"os"

	"github.com/mongodb/mongo-tools/common/log"
	"github.com/mongodb/mongo-tools/common/util"
	"github.com/mongodb/mongo-tools/mongoexport"
)

var (
	VersionStr = "build-without-version-string"
	GitCommit  = "build-without-git-commit"
)

func MongoExport(args []string) error {
	opts, err := mongoexport.ParseOptions(args, VersionStr, GitCommit)
	if err != nil {
		log.Logvf(log.Always, "error parsing options: %v", err)
		log.Logvf(log.Always, util.ShortUsage("mongoexport"))
		return err
	}
	exporter, err := mongoexport.New(opts)
	if err != nil {
		log.Logvf(log.Always, "%v", err)

		if se, ok := err.(util.SetupError); ok && se.Message != "" {
			log.Logv(log.Always, se.Message)
		}
		return err
	}
	defer exporter.Close()

	writer, err := exporter.GetOutputWriter()
	if err != nil {
		log.Logvf(log.Always, "error opening output stream: %v", err)
		return err
	}
	if writer == nil {
		writer = os.Stdout
	} else {
		defer writer.Close()
	}

	numDocs, err := exporter.Export(writer)
	if err != nil {
		log.Logvf(log.Always, "Failed: %v", err)
		return err
	}

	if numDocs == 1 {
		log.Logvf(log.Always, "exported %v record", numDocs)
	} else {
		log.Logvf(log.Always, "exported %v records", numDocs)
	}
	return nil
}

func MongoExportArgsBuilder(uri string, collection string, query string, out string) []string {
	args := make([]string, 3)
	args[0] = fmt.Sprintf("--uri %s", uri)
	args[1] = fmt.Sprintf("--collection %s", collection)
	args[2] = fmt.Sprintf("--query %s", query)
	args[3] = fmt.Sprintf("--out %s", out)
	return args
}
