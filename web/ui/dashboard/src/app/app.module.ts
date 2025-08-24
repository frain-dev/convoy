import {APP_INITIALIZER, NgModule} from '@angular/core';
import {BrowserModule} from '@angular/platform-browser';
import {NotificationComponent} from 'src/app/components/notification/notification.component';

import {AppRoutingModule} from './app-routing.module';
import {AppComponent} from './app.component';
import {NotificationModalComponent} from './components/notification-modal/notification-modal.component';
import {ConfigService} from './services/config/config.service';


export function initializeGoogleOAuth(configService: ConfigService) {
  return () => configService.getConfig();
}

@NgModule({
	declarations: [AppComponent],
	imports: [
		BrowserModule,
		AppRoutingModule,
		NotificationComponent,
		NotificationModalComponent
	],
	providers: [
		ConfigService,
		{
			provide: APP_INITIALIZER,
			useFactory: initializeGoogleOAuth,
			deps: [ConfigService],
			multi: true
		}
	],
	bootstrap: [AppComponent]
})
export class AppModule {}
