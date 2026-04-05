import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Public paths -- no auth required
  if (
    pathname.startsWith("/login") ||
    pathname.startsWith("/api/auth") ||
    pathname.startsWith("/_next") ||
    pathname === "/favicon.ico"
  ) {
    return NextResponse.next();
  }

  // Check for pm_session cookie
  const session = request.cookies.get("pm_session");
  if (!session) {
    // In self-hosted autologin mode, the Go backend will set the cookie
    // automatically on the first API call. For the frontend, we redirect
    // to login only in cloud mode. Check for auth_mode cookie or env.
    const authMode = process.env.NEXT_PUBLIC_AUTH_MODE || "autologin";
    if (authMode === "oauth") {
      return NextResponse.redirect(new URL("/login", request.url));
    }
    // In autologin mode, let the request through -- the API will auto-set cookies
    return NextResponse.next();
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
