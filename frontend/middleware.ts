import { NextResponse } from 'next/server';
import type { NextRequest } from 'next/server';

// Route konfigurációk
const protectedRoutes = {
    '/dashboard': { permissions: ['profile.read'] },
    '/users': { permissions: ['users.list', 'users.read'] },
    '/users/create': { permissions: ['users.create'] },
    '/admin': { role: 'admin' },
    '/settings': { permissions: ['system.settings'] }
};

export function middleware(request: NextRequest) {


    const token = request.cookies.get('auth_token')?.value ||
        request.headers.get('authorization')?.replace('Bearer ', '');

    const pathname = request.nextUrl.pathname;


    // Public routes
    if (pathname.startsWith('/auth') || pathname === '/') {
        return NextResponse.next();
    }

    // Ha nincs token és védett route
    if (!token && Object.keys(protectedRoutes).some(route => pathname.startsWith(route))) {
        return NextResponse.redirect(new URL('/auth', request.url));
    }

    // Itt lehetne JWT decode és permission ellenőrzés
    // De a frontend-en inkább a komponens szintű védelem a fő

    return NextResponse.next();
}

export const config = {
    matcher: [
        '/((?!api|_next/static|_next/image|favicon.ico).*)',
    ],
};