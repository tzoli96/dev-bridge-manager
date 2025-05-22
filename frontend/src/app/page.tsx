import HealthCheck from '@/components/HealthCheck'

export default function Home() {
  return (
      <main className="min-h-screen bg-gray-50 py-12 px-4">
        <div className="max-w-4xl mx-auto">
          <div className="text-center mb-12">
            <h1 className="text-4xl font-bold text-gray-900 mb-4">
              Dev Bridge Manager
            </h1>
            <p className="text-xl text-gray-600 mb-8">
              Multi-agency development project management system
            </p>
            <div className="inline-block bg-blue-100 border border-blue-400 text-blue-700 px-4 py-3 rounded">
              🚀 Frontend is running with Next.js 15 + Turbopack!
            </div>
          </div>

          {/* Health Check Component */}
          <div className="flex justify-center">
            <HealthCheck />
          </div>

          {/* Additional Info */}
          <div className="mt-12 text-center text-gray-500">
            <p>Test the connection between frontend and backend services</p>
          </div>
        </div>
      </main>
  )
}