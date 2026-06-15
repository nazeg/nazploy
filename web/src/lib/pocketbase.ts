import PocketBase from 'pocketbase'

const pb = new PocketBase(
  import.meta.env.VITE_PB_URL || window.location.origin
)

// Ensure the Authorization header is always sent with the current token.
// This guards against edge cases where the authStore token exists but
// the SDK doesn't attach it (e.g. after a version mismatch or SSR hydration).
pb.beforeSend = function (url, options) {
  if (pb.authStore.token && !options.headers?.['Authorization']) {
    options.headers = {
      ...options.headers,
      Authorization: pb.authStore.token,
    }
  }
  return { url, options }
}

export default pb
