export default defineNuxtPlugin(() => {
  const { loggedIn, login } = useAuth();

  return {
    provide: {
      platformReauth: async ({
        requireLoggedIn = true,
      }: { requireLoggedIn?: boolean } = {}) => {
        if (requireLoggedIn && !loggedIn.value) return false;

        await login();
        return true;
      },
    },
  };
});
