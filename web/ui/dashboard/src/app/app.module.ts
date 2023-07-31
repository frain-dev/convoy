import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { NotificationComponent } from 'src/app/components/notification/notification.component';

import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import { NotificationModalComponent } from './components/notification-modal/notification-modal.component';

@NgModule({
	declarations: [AppComponent],
	imports: [BrowserModule, AppRoutingModule, NotificationComponent, NotificationModalComponent],
	bootstrap: [AppComponent]
})
export class AppModule {}
