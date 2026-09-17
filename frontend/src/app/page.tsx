import HealthCheck from '../components/HealthCheck'
import Link from 'next/link'
import { Boxes, LogIn, Users } from 'lucide-react'

export default function Home() {
  return (
      <main className="min-h-screen bg-background py-12 px-4">
        <div className="max-w-4xl mx-auto">
          <div className="text-center mb-12">
            <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
              <Boxes size={24} />
            </div>
            <h1 className="text-4xl font-bold text-foreground mb-3">
              Dev Bridge Manager
            </h1>
            <p className="text-lg text-muted-foreground mb-6">
              Multi-agency development project management system
            </p>
            <div className="inline-block bg-primary/10 border border-primary/20 text-primary text-sm px-4 py-2 rounded-full">
              Frontend running on Next.js 15 + Turbopack
            </div>
          </div>

          {/* Navigation */}
          <div className="flex justify-center mb-12">
            <div className="bg-card rounded-xl shadow-sm border border-border p-6 w-full max-w-md">
              <h2 className="text-base font-semibold text-foreground mb-4 text-center">Quick Navigation</h2>
              <div className="grid grid-cols-2 gap-3">
                <Link
                    href="/auth"
                    className="flex items-center justify-center gap-2 bg-primary text-primary-foreground px-6 py-3 rounded-xl font-medium text-sm hover:bg-primary/90 active:scale-[0.98] transition-all"
                >
                  <LogIn size={16} />
                  Login / Register
                </Link>
                <Link
                    href="/users"
                    className="flex items-center justify-center gap-2 bg-card text-foreground border border-border px-6 py-3 rounded-xl font-medium text-sm hover:bg-muted active:scale-[0.98] transition-all"
                >
                  <Users size={16} />
                  View Users
                </Link>
              </div>
            </div>
          </div>

          {/* Health Check Component */}
          <div className="flex justify-center">
            <HealthCheck />
          </div>

          {/* Additional Info */}
          <div className="mt-12 text-center text-muted-foreground text-sm">
            <p>Test the connection between frontend and backend services</p>
          </div>
        </div>
      </main>
  )
}
