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
import { Route as ProjectsEndpointsIndexImport } from './app/projects_/endpoints/index'
import { Route as ProjectsEndpointsNewImport } from './app/projects_/endpoints/new'
import { Route as ProjectsEndpointsEndpointIdImport } from './app/projects_/endpoints/$endpointId'
import { Route as ProjectsProjectIdSettingsImport } from './app/projects_/$projectId/settings'
import { Route as ProjectsProjectIdSubscriptionsIndexImport } from './app/projects_/$projectId/subscriptions/index'
import { Route as ProjectsProjectIdSourcesIndexImport } from './app/projects_/$projectId/sources/index'
import { Route as ProjectsProjectIdEndpointsIndexImport } from './app/projects_/$projectId/endpoints/index'
import { Route as ProjectsProjectIdSubscriptionsNewImport } from './app/projects_/$projectId/subscriptions/new'
import { Route as ProjectsProjectIdSourcesNewImport } from './app/projects_/$projectId/sources/new'
import { Route as ProjectsProjectIdSourcesSourceIdImport } from './app/projects_/$projectId/sources/$sourceId'
import { Route as ProjectsProjectIdEndpointsNewImport } from './app/projects_/$projectId/endpoints/new'
import { Route as ProjectsProjectIdEndpointsEndpointIdImport } from './app/projects_/$projectId/endpoints/$endpointId'

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

const ProjectsEndpointsIndexRoute = ProjectsEndpointsIndexImport.update({
  id: '/projects_/endpoints/',
  path: '/projects/endpoints/',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsEndpointsNewRoute = ProjectsEndpointsNewImport.update({
  id: '/projects_/endpoints/new',
  path: '/projects/endpoints/new',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsEndpointsEndpointIdRoute =
  ProjectsEndpointsEndpointIdImport.update({
    id: '/projects_/endpoints/$endpointId',
    path: '/projects/endpoints/$endpointId',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdSettingsRoute = ProjectsProjectIdSettingsImport.update({
  id: '/projects_/$projectId/settings',
  path: '/projects/$projectId/settings',
  getParentRoute: () => rootRoute,
} as any)

const ProjectsProjectIdSubscriptionsIndexRoute =
  ProjectsProjectIdSubscriptionsIndexImport.update({
    id: '/projects_/$projectId/subscriptions/',
    path: '/projects/$projectId/subscriptions/',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdSourcesIndexRoute =
  ProjectsProjectIdSourcesIndexImport.update({
    id: '/projects_/$projectId/sources/',
    path: '/projects/$projectId/sources/',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdEndpointsIndexRoute =
  ProjectsProjectIdEndpointsIndexImport.update({
    id: '/projects_/$projectId/endpoints/',
    path: '/projects/$projectId/endpoints/',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdSubscriptionsNewRoute =
  ProjectsProjectIdSubscriptionsNewImport.update({
    id: '/projects_/$projectId/subscriptions/new',
    path: '/projects/$projectId/subscriptions/new',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdSourcesNewRoute =
  ProjectsProjectIdSourcesNewImport.update({
    id: '/projects_/$projectId/sources/new',
    path: '/projects/$projectId/sources/new',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdSourcesSourceIdRoute =
  ProjectsProjectIdSourcesSourceIdImport.update({
    id: '/projects_/$projectId/sources/$sourceId',
    path: '/projects/$projectId/sources/$sourceId',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdEndpointsNewRoute =
  ProjectsProjectIdEndpointsNewImport.update({
    id: '/projects_/$projectId/endpoints/new',
    path: '/projects/$projectId/endpoints/new',
    getParentRoute: () => rootRoute,
  } as any)

const ProjectsProjectIdEndpointsEndpointIdRoute =
  ProjectsProjectIdEndpointsEndpointIdImport.update({
    id: '/projects_/$projectId/endpoints/$endpointId',
    path: '/projects/$projectId/endpoints/$endpointId',
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
    '/projects_/endpoints/$endpointId': {
      id: '/projects_/endpoints/$endpointId'
      path: '/projects/endpoints/$endpointId'
      fullPath: '/projects/endpoints/$endpointId'
      preLoaderRoute: typeof ProjectsEndpointsEndpointIdImport
      parentRoute: typeof rootRoute
    }
    '/projects_/endpoints/new': {
      id: '/projects_/endpoints/new'
      path: '/projects/endpoints/new'
      fullPath: '/projects/endpoints/new'
      preLoaderRoute: typeof ProjectsEndpointsNewImport
      parentRoute: typeof rootRoute
    }
    '/projects_/endpoints/': {
      id: '/projects_/endpoints/'
      path: '/projects/endpoints'
      fullPath: '/projects/endpoints'
      preLoaderRoute: typeof ProjectsEndpointsIndexImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/endpoints/$endpointId': {
      id: '/projects_/$projectId/endpoints/$endpointId'
      path: '/projects/$projectId/endpoints/$endpointId'
      fullPath: '/projects/$projectId/endpoints/$endpointId'
      preLoaderRoute: typeof ProjectsProjectIdEndpointsEndpointIdImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/endpoints/new': {
      id: '/projects_/$projectId/endpoints/new'
      path: '/projects/$projectId/endpoints/new'
      fullPath: '/projects/$projectId/endpoints/new'
      preLoaderRoute: typeof ProjectsProjectIdEndpointsNewImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/sources/$sourceId': {
      id: '/projects_/$projectId/sources/$sourceId'
      path: '/projects/$projectId/sources/$sourceId'
      fullPath: '/projects/$projectId/sources/$sourceId'
      preLoaderRoute: typeof ProjectsProjectIdSourcesSourceIdImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/sources/new': {
      id: '/projects_/$projectId/sources/new'
      path: '/projects/$projectId/sources/new'
      fullPath: '/projects/$projectId/sources/new'
      preLoaderRoute: typeof ProjectsProjectIdSourcesNewImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/subscriptions/new': {
      id: '/projects_/$projectId/subscriptions/new'
      path: '/projects/$projectId/subscriptions/new'
      fullPath: '/projects/$projectId/subscriptions/new'
      preLoaderRoute: typeof ProjectsProjectIdSubscriptionsNewImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/endpoints/': {
      id: '/projects_/$projectId/endpoints/'
      path: '/projects/$projectId/endpoints'
      fullPath: '/projects/$projectId/endpoints'
      preLoaderRoute: typeof ProjectsProjectIdEndpointsIndexImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/sources/': {
      id: '/projects_/$projectId/sources/'
      path: '/projects/$projectId/sources'
      fullPath: '/projects/$projectId/sources'
      preLoaderRoute: typeof ProjectsProjectIdSourcesIndexImport
      parentRoute: typeof rootRoute
    }
    '/projects_/$projectId/subscriptions/': {
      id: '/projects_/$projectId/subscriptions/'
      path: '/projects/$projectId/subscriptions'
      fullPath: '/projects/$projectId/subscriptions'
      preLoaderRoute: typeof ProjectsProjectIdSubscriptionsIndexImport
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
  '/projects/endpoints/$endpointId': typeof ProjectsEndpointsEndpointIdRoute
  '/projects/endpoints/new': typeof ProjectsEndpointsNewRoute
  '/projects/endpoints': typeof ProjectsEndpointsIndexRoute
  '/projects/$projectId/endpoints/$endpointId': typeof ProjectsProjectIdEndpointsEndpointIdRoute
  '/projects/$projectId/endpoints/new': typeof ProjectsProjectIdEndpointsNewRoute
  '/projects/$projectId/sources/$sourceId': typeof ProjectsProjectIdSourcesSourceIdRoute
  '/projects/$projectId/sources/new': typeof ProjectsProjectIdSourcesNewRoute
  '/projects/$projectId/subscriptions/new': typeof ProjectsProjectIdSubscriptionsNewRoute
  '/projects/$projectId/endpoints': typeof ProjectsProjectIdEndpointsIndexRoute
  '/projects/$projectId/sources': typeof ProjectsProjectIdSourcesIndexRoute
  '/projects/$projectId/subscriptions': typeof ProjectsProjectIdSubscriptionsIndexRoute
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
  '/projects/endpoints/$endpointId': typeof ProjectsEndpointsEndpointIdRoute
  '/projects/endpoints/new': typeof ProjectsEndpointsNewRoute
  '/projects/endpoints': typeof ProjectsEndpointsIndexRoute
  '/projects/$projectId/endpoints/$endpointId': typeof ProjectsProjectIdEndpointsEndpointIdRoute
  '/projects/$projectId/endpoints/new': typeof ProjectsProjectIdEndpointsNewRoute
  '/projects/$projectId/sources/$sourceId': typeof ProjectsProjectIdSourcesSourceIdRoute
  '/projects/$projectId/sources/new': typeof ProjectsProjectIdSourcesNewRoute
  '/projects/$projectId/subscriptions/new': typeof ProjectsProjectIdSubscriptionsNewRoute
  '/projects/$projectId/endpoints': typeof ProjectsProjectIdEndpointsIndexRoute
  '/projects/$projectId/sources': typeof ProjectsProjectIdSourcesIndexRoute
  '/projects/$projectId/subscriptions': typeof ProjectsProjectIdSubscriptionsIndexRoute
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
  '/projects_/endpoints/$endpointId': typeof ProjectsEndpointsEndpointIdRoute
  '/projects_/endpoints/new': typeof ProjectsEndpointsNewRoute
  '/projects_/endpoints/': typeof ProjectsEndpointsIndexRoute
  '/projects_/$projectId/endpoints/$endpointId': typeof ProjectsProjectIdEndpointsEndpointIdRoute
  '/projects_/$projectId/endpoints/new': typeof ProjectsProjectIdEndpointsNewRoute
  '/projects_/$projectId/sources/$sourceId': typeof ProjectsProjectIdSourcesSourceIdRoute
  '/projects_/$projectId/sources/new': typeof ProjectsProjectIdSourcesNewRoute
  '/projects_/$projectId/subscriptions/new': typeof ProjectsProjectIdSubscriptionsNewRoute
  '/projects_/$projectId/endpoints/': typeof ProjectsProjectIdEndpointsIndexRoute
  '/projects_/$projectId/sources/': typeof ProjectsProjectIdSourcesIndexRoute
  '/projects_/$projectId/subscriptions/': typeof ProjectsProjectIdSubscriptionsIndexRoute
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
    | '/projects/endpoints/$endpointId'
    | '/projects/endpoints/new'
    | '/projects/endpoints'
    | '/projects/$projectId/endpoints/$endpointId'
    | '/projects/$projectId/endpoints/new'
    | '/projects/$projectId/sources/$sourceId'
    | '/projects/$projectId/sources/new'
    | '/projects/$projectId/subscriptions/new'
    | '/projects/$projectId/endpoints'
    | '/projects/$projectId/sources'
    | '/projects/$projectId/subscriptions'
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
    | '/projects/endpoints/$endpointId'
    | '/projects/endpoints/new'
    | '/projects/endpoints'
    | '/projects/$projectId/endpoints/$endpointId'
    | '/projects/$projectId/endpoints/new'
    | '/projects/$projectId/sources/$sourceId'
    | '/projects/$projectId/sources/new'
    | '/projects/$projectId/subscriptions/new'
    | '/projects/$projectId/endpoints'
    | '/projects/$projectId/sources'
    | '/projects/$projectId/subscriptions'
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
    | '/projects_/endpoints/$endpointId'
    | '/projects_/endpoints/new'
    | '/projects_/endpoints/'
    | '/projects_/$projectId/endpoints/$endpointId'
    | '/projects_/$projectId/endpoints/new'
    | '/projects_/$projectId/sources/$sourceId'
    | '/projects_/$projectId/sources/new'
    | '/projects_/$projectId/subscriptions/new'
    | '/projects_/$projectId/endpoints/'
    | '/projects_/$projectId/sources/'
    | '/projects_/$projectId/subscriptions/'
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
  ProjectsEndpointsEndpointIdRoute: typeof ProjectsEndpointsEndpointIdRoute
  ProjectsEndpointsNewRoute: typeof ProjectsEndpointsNewRoute
  ProjectsEndpointsIndexRoute: typeof ProjectsEndpointsIndexRoute
  ProjectsProjectIdEndpointsEndpointIdRoute: typeof ProjectsProjectIdEndpointsEndpointIdRoute
  ProjectsProjectIdEndpointsNewRoute: typeof ProjectsProjectIdEndpointsNewRoute
  ProjectsProjectIdSourcesSourceIdRoute: typeof ProjectsProjectIdSourcesSourceIdRoute
  ProjectsProjectIdSourcesNewRoute: typeof ProjectsProjectIdSourcesNewRoute
  ProjectsProjectIdSubscriptionsNewRoute: typeof ProjectsProjectIdSubscriptionsNewRoute
  ProjectsProjectIdEndpointsIndexRoute: typeof ProjectsProjectIdEndpointsIndexRoute
  ProjectsProjectIdSourcesIndexRoute: typeof ProjectsProjectIdSourcesIndexRoute
  ProjectsProjectIdSubscriptionsIndexRoute: typeof ProjectsProjectIdSubscriptionsIndexRoute
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
  ProjectsEndpointsEndpointIdRoute: ProjectsEndpointsEndpointIdRoute,
  ProjectsEndpointsNewRoute: ProjectsEndpointsNewRoute,
  ProjectsEndpointsIndexRoute: ProjectsEndpointsIndexRoute,
  ProjectsProjectIdEndpointsEndpointIdRoute:
    ProjectsProjectIdEndpointsEndpointIdRoute,
  ProjectsProjectIdEndpointsNewRoute: ProjectsProjectIdEndpointsNewRoute,
  ProjectsProjectIdSourcesSourceIdRoute: ProjectsProjectIdSourcesSourceIdRoute,
  ProjectsProjectIdSourcesNewRoute: ProjectsProjectIdSourcesNewRoute,
  ProjectsProjectIdSubscriptionsNewRoute:
    ProjectsProjectIdSubscriptionsNewRoute,
  ProjectsProjectIdEndpointsIndexRoute: ProjectsProjectIdEndpointsIndexRoute,
  ProjectsProjectIdSourcesIndexRoute: ProjectsProjectIdSourcesIndexRoute,
  ProjectsProjectIdSubscriptionsIndexRoute:
    ProjectsProjectIdSubscriptionsIndexRoute,
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
        "/projects_/endpoints/$endpointId",
        "/projects_/endpoints/new",
        "/projects_/endpoints/",
        "/projects_/$projectId/endpoints/$endpointId",
        "/projects_/$projectId/endpoints/new",
        "/projects_/$projectId/sources/$sourceId",
        "/projects_/$projectId/sources/new",
        "/projects_/$projectId/subscriptions/new",
        "/projects_/$projectId/endpoints/",
        "/projects_/$projectId/sources/",
        "/projects_/$projectId/subscriptions/"
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
    "/projects_/endpoints/$endpointId": {
      "filePath": "projects_/endpoints/$endpointId.tsx"
    },
    "/projects_/endpoints/new": {
      "filePath": "projects_/endpoints/new.tsx"
    },
    "/projects_/endpoints/": {
      "filePath": "projects_/endpoints/index.tsx"
    },
    "/projects_/$projectId/endpoints/$endpointId": {
      "filePath": "projects_/$projectId/endpoints/$endpointId.tsx"
    },
    "/projects_/$projectId/endpoints/new": {
      "filePath": "projects_/$projectId/endpoints/new.tsx"
    },
    "/projects_/$projectId/sources/$sourceId": {
      "filePath": "projects_/$projectId/sources/$sourceId.tsx"
    },
    "/projects_/$projectId/sources/new": {
      "filePath": "projects_/$projectId/sources/new.tsx"
    },
    "/projects_/$projectId/subscriptions/new": {
      "filePath": "projects_/$projectId/subscriptions/new.tsx"
    },
    "/projects_/$projectId/endpoints/": {
      "filePath": "projects_/$projectId/endpoints/index.tsx"
    },
    "/projects_/$projectId/sources/": {
      "filePath": "projects_/$projectId/sources/index.tsx"
    },
    "/projects_/$projectId/subscriptions/": {
      "filePath": "projects_/$projectId/subscriptions/index.tsx"
    }
  }
}
ROUTE_MANIFEST_END */
