/*
Copyright 2021 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package directimagestreammigration

// PhaseDescriptions are human readable strings that describe a phase
var PhaseDescriptions = map[string]string{
	Created:            "DirectImageStreamMigration created.",
	Started:            "DirectImageStreamMigration started.",
	Prepare:            "Preparing for DirectImageStreamMigration.",
	MigrateImageStream: "Migrating images found in the ImageStream from source to target cluster.",
	MigrationFailed:    "Migration failed.",
	Completed:          "Migration completed.",
}
