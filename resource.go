// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Resource safety primitives for exception-safe resource management.
// These provide the minimal interface for bracketed resource handling.

// Bracket provides exception-safe resource acquisition and release.
// This follows the bracket pattern: acquire → use → release, where release
// is guaranteed to run even if use raises an error.
//
// Returns Either containing the result or the error.
func Bracket[E, R, A any](
	acquire Cont[Resumed, R],
	release func(R) Cont[Resumed, struct{}],
	use func(R) Cont[Resumed, A],
) Cont[Resumed, Either[E, A]] {
	return Bind(acquire, func(resource R) Cont[Resumed, Either[E, A]] {
		// Run the use function and catch any errors
		result := RunError[E, A](use(resource))

		// Always release the resource
		return Bind(release(resource), func(_ struct{}) Cont[Resumed, Either[E, A]] {
			return Return[Resumed](result)
		})
	})
}

// OnError runs cleanup only if the computation throws an error.
func OnError[E, A any](
	body Cont[Resumed, A],
	cleanup func(E) Cont[Resumed, struct{}],
) Cont[Resumed, A] {
	return CatchError[E, A](body, func(e E) Cont[Resumed, A] {
		return Bind(cleanup(e), func(_ struct{}) Cont[Resumed, A] {
			return ThrowError[E, A](e) // Re-throw after cleanup
		})
	})
}
