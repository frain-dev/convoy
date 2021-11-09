import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule, Routes } from '@angular/router';
import { DashboardComponent } from './dashboard.component';
import { SharedModule } from 'src/app/shared/shared.module';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { MatNativeDateModule } from '@angular/material/core';

const routes: Routes = [{ path: '', component: DashboardComponent }];

@NgModule({
	declarations: [DashboardComponent],
	imports: [CommonModule, RouterModule.forChild(routes), SharedModule, FormsModule, ReactiveFormsModule, MatDatepickerModule, MatNativeDateModule],
	providers: [MatDatepickerModule]
})
export class DashboardModule {}
