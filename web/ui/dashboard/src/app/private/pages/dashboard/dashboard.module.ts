import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, Routes } from '@angular/router';
import { DashboardComponent } from './dashboard.component';
import { ConvoyDashboardModule } from 'convoy-dashboard';

const routes: Routes = [{ path: '', component: DashboardComponent }];

@NgModule({
	declarations: [DashboardComponent],
	imports: [CommonModule, RouterModule.forChild(routes), ConvoyDashboardModule],
	providers: []
})
export class DashboardModule {}
