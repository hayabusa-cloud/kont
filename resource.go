// ©Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package kont

// Bracket runs use(resource) under [RunError] after acquire succeeds.
// It calls release when use returns normally or raises a matching [Throw].
// A Go panic escaping use skips release. Handle other effect families inside use.
//
// The outer handler drives effects from acquire and release, including their errors.
// When release completes, Bracket returns the [Either] from use. Discarding a
// suspension during release skips its remaining callbacks.
func Bracket[E, R, A any](
	acquire Cont[Resumed, R],
	release func(R) Cont[Resumed, struct{}],
	use func(R) Cont[Resumed, A],
) Cont[Resumed, Either[E, A]] {
	return Bind(acquire, func(resource R) Cont[Resumed, Either[E, A]] {
		result := RunError[E, A](use(resource))

		return Bind(release(resource), func(_ struct{}) Cont[Resumed, Either[E, A]] {
			return Return[Resumed](result)
		})
	})
}

// OnError runs cleanup when [CatchError] catches an error of type E from body.
// The standard error handler runs both body and cleanup under [RunError].
// It rethrows the original error if cleanup succeeds. If cleanup raises an
// error of type E, that error propagates.
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
