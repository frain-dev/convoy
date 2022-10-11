import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { NotificationComponent } from 'src/app/components/notification/notification.component';

import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import { SuccessModalComponent } from './components/success-modal/success-modal.component';

@NgModule({
	declarations: [AppComponent],
	imports: [BrowserModule, AppRoutingModule, NotificationComponent, SuccessModalComponent],
	bootstrap: [AppComponent]
})
export class AppModule {}
