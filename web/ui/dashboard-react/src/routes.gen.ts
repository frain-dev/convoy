/* eslint-disable */

// @ts-nocheck

// noinspection JSUnusedGlobalSymbols

// This file was automatically generated by TanStack Router.
// You should NOT make any changes in this file as it will be overwritten.
// Additionally, you should also exclude this file from your linter and/or formatter to prevent it from being checked or modified.

// Import Routes

import { Route as rootRoute } from './app/__root'
import { Route as UserSettingsImport } from './app/user-settings'
import { Route as SignupImport } from './app/signup'
import { Route as SettingsImport } from './app/settings'
import { Route as LoginImport } from './app/login'
import { Route as GetStartedImport } from './app/get-started'
import { Route as ForgotPasswordImport } from './app/forgot-password'
import { Route as ProjectsRouteImport } from './app/projects/route'
import { Route as IndexImport } from './app/index'
import { Route as ProjectsIndexImport } from './app/projects/index'
import { Route as ProjectsNewImport } from './app/projects_/new'
import { Route as ProjectsProjectIdSettingsImport } from './app/projects_/$projectId/settings'
import { Route as ProjectsProjectIdEndpointsNewImport } from './app/projects_/$projectId/endpoints/new'

// Create/Update Routes

const UserSettingsRoute = UserSettingsImport.update({
  id: '/user-settings',
  path: '/user-settings',
  getParentRoute: () => rootRoute,
} as any)

const SignupRoute = SignupImport.update({
  id: '/signup',
  path: '/signup',
  getParentRoute: () => rootRoute,
} as any)

const SettingsRoute = SettingsImport.update({
  id: '/settings',
  path: '/settings',
  getParentRoute: () => rootRoute,
} as any)

const LoginRoute = LoginImport.update({
  id: '/login',
  path: '/login',
  getParentRoute: () => rootRoute,
} as any)

const GetStartedRoute = GetStartedImport.update({
  id: '/get-started',
  path: '/get-started',
  getParentRoute: () => rootRoute,
} as any)

const ForgotPasswordRoute = ForgotPasswordImport.update({
  id: '/forgot-password',
  path: '/forgot-password',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsRouteRoute = ProjectsRouteImport.update({
  id: '/projects',
  path: '/projects',
  getParentRoute: () => rootRoute,
} as any)

const IndexRoute = IndexImport.update({
  id: '/',
  path: '/',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsIndexRoute = ProjectsIndexImport.update({
  id: '/',
  path: '/',
  getParentRoute: () => ProjectsRouteRoute,
} as any)

const ProjectsNewRoute = ProjectsNewImport.update({
  id: '/projects_/new',
  path: '/projects/new',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsProjectIdSettingsRoute = ProjectsProjectIdSettingsImport.update({
  id: '/projects_/$projectId/settings',
  path: '/projects/$projectId/settings',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsProjectIdEndpointsNewRoute =
  ProjectsProjectIdEndpointsNewImport.update({
    id: '/projects_/$projectId/endpoints/new',
    path: '/projects/$projectId/endpoints/new',
    getParentRoute: () => rootRoute,
  } as any)

// Populate the FileRoutesByPath interface

declare module '@tanstack/react-router' {
  interface FileRoutesByPath {
    '/': {
      id: '/'
      path: '/'
      fullPath: '/'
      preLoaderRoute: typeof IndexImport
      parentRoute: typeof rootRoute
    }
    '/projects': {
      id: '/projects'
      path: '/projects'
      fullPath: '/projects'
      preLoaderRoute: typeof ProjectsRouteImport
      parentRoute: typeof rootRoute
    }
    '/forgot-password': {
      id: '/forgot-password'
      path: '/forgot-password'
      fullPath: '/forgot-password'
      preLoaderRoute: typeof ForgotPasswordImport
      parentRoute: typeof rootRoute
    }
    '/get-started': {
      id: '/get-started'
      path: '/get-started'
      fullPath: '/get-started'
      preLoaderRoute: typeof GetStartedImport
      parentRoute: typeof rootRoute
    }
    '/login': {
      id: '/login'
      path: '/login'
      fullPath: '/login'
      preLoaderRoute: typeof LoginImport
      parentRoute: typeof rootRoute
    }
    '/settings': {
      id: '/settings'
      path: '/settings'
      fullPath: '/settings'
      preLoaderRoute: typeof SettingsImport
      parentRoute: typeof rootRoute
    }
    '/signup': {
      id: '/signup'
      path: '/signup'
      fullPath: '/signup'
      preLoaderRoute: typeof SignupImport
      parentRoute: typeof rootRoute
    }
    '/user-settings': {
      id: '/user-settings'
      path: '/user-settings'
      fullPath: '/user-settings'
      preLoaderRoute: typeof UserSettingsImport
      parentRoute: typeof rootRoute
    }
    '/projects_/new': {
      id: '/projects_/new'
      path: '/projects/new'
      fullPath: '/projects/new'
      preLoaderRoute: typeof ProjectsNewImport
      parentRoute: typeof rootRoute
    }
    '/projects/': {
      id: '/projects/'
      path: '/'
      fullPath: '/projects/'
      preLoaderRoute: typeof ProjectsIndexImport
      parentRoute: typeof ProjectsRouteImport
    }
    '/projects_/$projectId/settings': {
      id: '/projects_/$projectId/settings'
      path: '/projects/$projectId/settings'
      fullPath: '/projects/$projectId/settings'
      preLoaderRoute: typeof ProjectsProjectIdSettingsImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/endpoints/new': {
      id: '/projects_/$projectId/endpoints/new'
      path: '/projects/$projectId/endpoints/new'
      fullPath: '/projects/$projectId/endpoints/new'
      preLoaderRoute: typeof ProjectsProjectIdEndpointsNewImport
      parentRoute: typeof rootRoute
    }
  }
}

// Create and export the route tree

interface ProjectsRouteRouteChildren {
  ProjectsIndexRoute: typeof ProjectsIndexRoute
}

const ProjectsRouteRouteChildren: ProjectsRouteRouteChildren = {
  ProjectsIndexRoute: ProjectsIndexRoute,
}

const ProjectsRouteRouteWithChildren = ProjectsRouteRoute._addFileChildren(
  ProjectsRouteRouteChildren,
)

export interface FileRoutesByFullPath {
  '/': typeof IndexRoute
  '/projects': typeof ProjectsRouteRouteWithChildren
  '/forgot-password': typeof ForgotPasswordRoute
  '/get-started': typeof GetStartedRoute
  '/login': typeof LoginRoute
  '/settings': typeof SettingsRoute
  '/signup': typeof SignupRoute
  '/user-settings': typeof UserSettingsRoute
  '/projects/new': typeof ProjectsNewRoute
  '/projects/': typeof ProjectsIndexRoute
  '/projects/$projectId/settings': typeof ProjectsProjectIdSettingsRoute
  '/projects/$projectId/endpoints/new': typeof ProjectsProjectIdEndpointsNewRoute
}

export interface FileRoutesByTo {
  '/': typeof IndexRoute
  '/forgot-password': typeof ForgotPasswordRoute
  '/get-started': typeof GetStartedRoute
  '/login': typeof LoginRoute
  '/settings': typeof SettingsRoute
  '/signup': typeof SignupRoute
  '/user-settings': typeof UserSettingsRoute
  '/projects/new': typeof ProjectsNewRoute
  '/projects': typeof ProjectsIndexRoute
  '/projects/$projectId/settings': typeof ProjectsProjectIdSettingsRoute
  '/projects/$projectId/endpoints/new': typeof ProjectsProjectIdEndpointsNewRoute
}

export interface FileRoutesById {
  __root__: typeof rootRoute
  '/': typeof IndexRoute
  '/projects': typeof ProjectsRouteRouteWithChildren
  '/forgot-password': typeof ForgotPasswordRoute
  '/get-started': typeof GetStartedRoute
  '/login': typeof LoginRoute
  '/settings': typeof SettingsRoute
  '/signup': typeof SignupRoute
  '/user-settings': typeof UserSettingsRoute
  '/projects_/new': typeof ProjectsNewRoute
  '/projects/': typeof ProjectsIndexRoute
  '/projects_/$projectId/settings': typeof ProjectsProjectIdSettingsRoute
  '/projects_/$projectId/endpoints/new': typeof ProjectsProjectIdEndpointsNewRoute
}

export interface FileRouteTypes {
  fileRoutesByFullPath: FileRoutesByFullPath
  fullPaths:
    | '/'
    | '/projects'
    | '/forgot-password'
    | '/get-started'
    | '/login'
    | '/settings'
    | '/signup'
    | '/user-settings'
    | '/projects/new'
    | '/projects/'
    | '/projects/$projectId/settings'
    | '/projects/$projectId/endpoints/new'
  fileRoutesByTo: FileRoutesByTo
  to:
    | '/'
    | '/forgot-password'
    | '/get-started'
    | '/login'
    | '/settings'
    | '/signup'
    | '/user-settings'
    | '/projects/new'
    | '/projects'
    | '/projects/$projectId/settings'
    | '/projects/$projectId/endpoints/new'
  id:
    | '__root__'
    | '/'
    | '/projects'
    | '/forgot-password'
    | '/get-started'
    | '/login'
    | '/settings'
    | '/signup'
    | '/user-settings'
    | '/projects_/new'
    | '/projects/'
    | '/projects_/$projectId/settings'
    | '/projects_/$projectId/endpoints/new'
  fileRoutesById: FileRoutesById
}

export interface RootRouteChildren {
  IndexRoute: typeof IndexRoute
  ProjectsRouteRoute: typeof ProjectsRouteRouteWithChildren
  ForgotPasswordRoute: typeof ForgotPasswordRoute
  GetStartedRoute: typeof GetStartedRoute
  LoginRoute: typeof LoginRoute
  SettingsRoute: typeof SettingsRoute
  SignupRoute: typeof SignupRoute
  UserSettingsRoute: typeof UserSettingsRoute
  ProjectsNewRoute: typeof ProjectsNewRoute
  ProjectsProjectIdSettingsRoute: typeof ProjectsProjectIdSettingsRoute
  ProjectsProjectIdEndpointsNewRoute: typeof ProjectsProjectIdEndpointsNewRoute
}

const rootRouteChildren: RootRouteChildren = {
  IndexRoute: IndexRoute,
  ProjectsRouteRoute: ProjectsRouteRouteWithChildren,
  ForgotPasswordRoute: ForgotPasswordRoute,
  GetStartedRoute: GetStartedRoute,
  LoginRoute: LoginRoute,
  SettingsRoute: SettingsRoute,
  SignupRoute: SignupRoute,
  UserSettingsRoute: UserSettingsRoute,
  ProjectsNewRoute: ProjectsNewRoute,
  ProjectsProjectIdSettingsRoute: ProjectsProjectIdSettingsRoute,
  ProjectsProjectIdEndpointsNewRoute: ProjectsProjectIdEndpointsNewRoute,
}

export const routeTree = rootRoute
  ._addFileChildren(rootRouteChildren)
  ._addFileTypes<FileRouteTypes>()

/* ROUTE_MANIFEST_START
{
  "routes": {
    "__root__": {
      "filePath": "__root.tsx",
      "children": [
        "/",
        "/projects",
        "/forgot-password",
        "/get-started",
        "/login",
        "/settings",
        "/signup",
        "/user-settings",
        "/projects_/new",
        "/projects_/$projectId/settings",
        "/projects_/$projectId/endpoints/new"
      ]
    },
    "/": {
      "filePath": "index.tsx"
    },
    "/projects": {
      "filePath": "projects/route.tsx",
      "children": [
        "/projects/"
      ]
    },
    "/forgot-password": {
      "filePath": "forgot-password.tsx"
    },
    "/get-started": {
      "filePath": "get-started.tsx"
    },
    "/login": {
      "filePath": "login.tsx"
    },
    "/settings": {
      "filePath": "settings.tsx"
    },
    "/signup": {
      "filePath": "signup.tsx"
    },
    "/user-settings": {
      "filePath": "user-settings.tsx"
    },
    "/projects_/new": {
      "filePath": "projects_/new.tsx"
    },
    "/projects/": {
      "filePath": "projects/index.tsx",
      "parent": "/projects"
    },
    "/projects_/$projectId/settings": {
      "filePath": "projects_/$projectId/settings.tsx"
    },
    "/projects_/$projectId/endpoints/new": {
      "filePath": "projects_/$projectId/endpoints/new.tsx"
    }
  }
}
ROUTE_MANIFEST_END */
