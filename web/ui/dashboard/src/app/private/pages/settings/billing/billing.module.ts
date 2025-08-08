import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { BillingPageComponent } from './billing-page.component';
import { BillingOverviewComponent } from './billing-overview.component';
import { BillingUsageComponent } from './billing-usage.component';
import { BillingInvoicesComponent } from './billing-invoices.component';
import {CardComponent} from "../../../../components/card/card.component";
import {
    TableCellComponent, TableComponent,
    TableHeadCellComponent, TableHeadComponent,
    TableRowComponent
} from "../../../../components/table/table.component";
import {TableLoaderModule} from "../../../components/table-loader/table-loader.module";
import {BadgeComponent} from "../../../../components/badge/badge.component";
import {ButtonComponent} from "../../../../components/button/button.component";
import {DropdownComponent, DropdownOptionDirective} from "../../../../components/dropdown/dropdown.component";
import {PermissionDirective} from "../../../components/permission/permission.directive";
import {RolePipe} from "../../../../pipes/role/role.pipe";
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from "../../../../components/input/input.component";
import { DialogDirective, DialogHeaderComponent } from "../../../../components/dialog/dialog.directive";

@NgModule({
  declarations: [
    BillingPageComponent,
    BillingOverviewComponent,
    BillingUsageComponent,
    BillingInvoicesComponent
  ],
    imports: [CommonModule, ReactiveFormsModule, CardComponent, TableRowComponent, TableCellComponent, TableHeadCellComponent, TableComponent, TableHeadComponent, TableLoaderModule, BadgeComponent, ButtonComponent, DropdownComponent, DropdownOptionDirective, PermissionDirective, RolePipe, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, DialogDirective, DialogHeaderComponent],
  exports: [
    BillingPageComponent,
    BillingOverviewComponent,
    BillingUsageComponent,
    BillingInvoicesComponent
  ]
})
export class BillingModule {}
