import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, Routes } from '@angular/router';
import { AppComponent } from './app.component';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { AppPortalModule } from '../app-portal/app-portal.module';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [CommonModule, RouterModule.forChild(routes), AppPortalModule],
	providers: [MatDatepickerModule]
})
export class AppModule {}
