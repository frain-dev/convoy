import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, Routes } from '@angular/router';
import { AppComponent } from './app.component';

const routes: Routes = [{ path: '', component: AppComponent }];

@NgModule({
	declarations: [AppComponent],
	imports: [CommonModule, RouterModule.forChild(routes)],
	providers: []
})
export class AppModule {}
