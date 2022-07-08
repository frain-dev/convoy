package util

import (
	"os"

	"github.com/mongodb/mongo-tools/common/log"
	"github.com/mongodb/mongo-tools/common/util"
	"github.com/mongodb/mongo-tools/mongoexport"
)

var (
	VersionStr = "build-without-version-string"
	GitCommit  = "build-without-git-commit"
)

func MongoExport(args []string) (int64, error) {
	opts, err := mongoexport.ParseOptions(args, VersionStr, GitCommit)
	if err != nil {
		log.Logvf(log.Always, "error parsing options: %v", err)
		log.Logvf(log.Always, util.ShortUsage("mongoexport"))
		return 0, err
	}
	exporter, err := mongoexport.New(opts)
	if err != nil {
		log.Logvf(log.Always, "%v", err)

		if se, ok := err.(util.SetupError); ok && se.Message != "" {
			log.Logv(log.Always, se.Message)
		}
		return 0, err
	}
	defer exporter.Close()

	writer, err := exporter.GetOutputWriter()
	if err != nil {
		log.Logvf(log.Always, "error opening output stream: %v", err)
		return 0, err
	}
	if writer == nil {
		writer = os.Stdout
	} else {
		defer writer.Close()
	}

	numDocs, err := exporter.Export(writer)
	if err != nil {
		log.Logvf(log.Always, "Failed: %v", err)
		return 0, err
	}

	if numDocs == 1 {
		log.Logvf(log.Always, "exported %v record", numDocs)
	} else {
		log.Logvf(log.Always, "exported %v records", numDocs)
	}
	return numDocs, nil
}

func MongoExportArgsBuilder(uri string, collection string, query string, out string) []string {
	args := make([]string, 8)
	args[0] = "--uri"
	args[1] = uri
	args[2] = "--collection"
	args[3] = collection
	args[4] = "--query"
	args[5] = query
	args[6] = "--out"
	args[7] = out
	return args
}
