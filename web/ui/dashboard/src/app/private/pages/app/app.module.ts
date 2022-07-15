import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, Routes } from '@angular/router';
import { AppComponent } from './app.component';
import { MatDatepickerModule } from '@angular/material/datepicker';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [CommonModule, RouterModule.forChild(routes)],
	providers: [MatDatepickerModule]
})
export class AppModule {}
