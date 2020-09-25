/*

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

package resources

// Result encapsulates the result of the work performed by a resource update.
type Result struct {
	err     error
	requeue bool
}

// Err returns any error associated with the result.
func (r Result) Err() error {
	return r.err
}

// Requeue returns true if a requeue is needed.
func (r Result) Requeue() bool {
	return r.requeue
}

var (
	// Done represents a result that is complete.
	Done = Result{}
	// Requeue is a result that needs to be re-queued.
	Requeue = Result{requeue: true}
)
