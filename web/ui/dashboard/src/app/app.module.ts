import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { HttpClientModule, HTTP_INTERCEPTORS } from '@angular/common/http';

import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import { HttpIntercepter } from './interceptor/http.interceptor';
import { ComponentsModule } from './components/components.module';
import { BrowserAnimationsModule } from '@angular/platform-browser/animations';

@NgModule({
	declarations: [AppComponent],
	imports: [BrowserModule, AppRoutingModule, HttpClientModule, ComponentsModule, BrowserAnimationsModule],
	providers: [{ provide: HTTP_INTERCEPTORS, useClass: HttpIntercepter, multi: true }],
	bootstrap: [AppComponent]
})
export class AppModule {}
